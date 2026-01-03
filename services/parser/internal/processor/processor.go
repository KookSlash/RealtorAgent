package processor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/sylvain/realtoragent/services/parser/internal/config"
	"github.com/sylvain/realtoragent/services/parser/internal/db"
	"github.com/sylvain/realtoragent/services/parser/internal/decision"
	"github.com/sylvain/realtoragent/services/parser/internal/message"
	"github.com/sylvain/realtoragent/services/parser/internal/model"
	"github.com/sylvain/realtoragent/services/parser/internal/normalize"
	"github.com/sylvain/realtoragent/services/parser/internal/vault"
)

type Processor struct {
	cfg       config.Config
	s3Client  *s3.Client
	sqsClient *sqs.Client
	db        *db.DB
	queueURL  string
	logger    *logger
}

type logger struct {
	inner *log.Logger
	level string
}

func newLogger(level string) *logger {
	return &logger{inner: log.New(os.Stdout, "", log.LstdFlags), level: strings.ToLower(level)}
}

func (l *logger) Debugf(format string, args ...interface{}) {
	if l.level == "debug" {
		l.inner.Printf("DEBUG: "+format, args...)
	}
}

func (l *logger) Infof(format string, args ...interface{}) {
	l.inner.Printf("INFO: "+format, args...)
}

func (l *logger) Warnf(format string, args ...interface{}) {
	l.inner.Printf("WARN: "+format, args...)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	l.inner.Printf("ERROR: "+format, args...)
}

func New(ctx context.Context, cfg config.Config, s3Client *s3.Client, sqsClient *sqs.Client, dbClient *db.DB) (*Processor, error) {
	queueURL, err := getQueueURL(ctx, sqsClient, cfg.RawEventsQueue)
	if err != nil {
		return nil, err
	}

	return &Processor{
		cfg:       cfg,
		s3Client:  s3Client,
		sqsClient: sqsClient,
		db:        dbClient,
		queueURL:  queueURL,
		logger:    newLogger(cfg.LogLevel),
	}, nil
}

func getQueueURL(ctx context.Context, client *sqs.Client, name string) (string, error) {
	out, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(name)})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.QueueUrl), nil
}

func (p *Processor) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if err := p.ProcessOnce(ctx); err != nil {
			p.logger.Errorf("process loop error: %v", err)
		}
	}
}

func (p *Processor) ProcessOnce(ctx context.Context) error {
	input := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(p.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     5,
	}
	if p.cfg.SQSVisibilityTimeout > 0 {
		input.VisibilityTimeout = p.cfg.SQSVisibilityTimeout
	}

	out, err := p.sqsClient.ReceiveMessage(ctx, input)
	if err != nil {
		return err
	}
	if len(out.Messages) == 0 {
		return nil
	}

	var firstErr error
	for _, msg := range out.Messages {
		deleteMessage, err := p.handleMessage(ctx, msg)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if deleteMessage {
			if err := p.deleteMessage(ctx, aws.ToString(msg.ReceiptHandle)); err != nil {
				p.logger.Errorf("failed to delete message: %v", err)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}

	return firstErr
}

func (p *Processor) deleteMessage(ctx context.Context, receiptHandle string) error {
	_, err := p.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(p.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	return err
}

func (p *Processor) handleMessage(ctx context.Context, msg types.Message) (bool, error) {
	body := aws.ToString(msg.Body)
	result := message.ParseSQSMessage(body)
	if result.Action == message.ActionDelete {
		p.logger.Infof("ignoring SQS message (%s)", result.Reason)
		return true, nil
	}

	for _, record := range result.Records {
		resultErr := p.processRecord(ctx, record.Bucket, record.Key)
		if resultErr != nil {
			return false, resultErr
		}
	}

	return true, nil
}

func (p *Processor) processRecord(ctx context.Context, bucket, key string) error {
	head, err := p.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("head object failed for %s/%s: %w", bucket, key, err)
	}
	etag := strings.Trim(aws.ToString(head.ETag), "\"")
	size := aws.ToInt64(head.ContentLength)

	processed, err := p.db.ProcessedExists(ctx, bucket, key, etag)
	if err != nil {
		return fmt.Errorf("processed_files lookup failed: %w", err)
	}
	if decision.ShouldSkipProcessed(processed) {
		p.logger.Infof("already processed %s/%s (etag %s)", bucket, key, etag)
		return nil
	}

	attempts, err := p.db.EnsureAttempt(ctx, bucket, key, etag)
	if err != nil {
		return fmt.Errorf("processing_attempts lookup failed: %w", err)
	}
	if !decision.ShouldProcessAttempt(attempts, p.cfg.MaxRetries) {
		p.logger.Warnf("max retries reached for %s/%s (etag %s)", bucket, key, etag)
		return nil
	}

	if p.cfg.FailOnKey != "" && strings.Contains(key, p.cfg.FailOnKey) {
		return p.handleFailure(ctx, bucket, key, etag, size, fmt.Errorf("forced failure for key %s", key), nil)
	}

	object, err := p.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return p.handleFailure(ctx, bucket, key, etag, size, fmt.Errorf("get object failed: %w", err), nil)
	}
	defer object.Body.Close()

	normFile, err := os.CreateTemp("", "normalized-*.jsonl")
	if err != nil {
		return p.handleFailure(ctx, bucket, key, etag, size, fmt.Errorf("create normalized temp file failed: %w", err), nil)
	}
	defer func() {
		_ = normFile.Close()
		_ = os.Remove(normFile.Name())
	}()

	errFile, err := os.CreateTemp("", "errors-*.jsonl")
	if err != nil {
		return p.handleFailure(ctx, bucket, key, etag, size, fmt.Errorf("create errors temp file failed: %w", err), nil)
	}
	defer func() {
		_ = errFile.Close()
		_ = os.Remove(errFile.Name())
	}()

	normWriter := bufio.NewWriter(normFile)
	errWriter := bufio.NewWriter(errFile)

	records, parseErr := p.parseJSONL(object.Body, normWriter, errWriter)
	if parseErr != nil {
		_ = normWriter.Flush()
		_ = errWriter.Flush()
		return p.handleFailure(ctx, bucket, key, etag, size, parseErr, errFile)
	}
	if err := normWriter.Flush(); err != nil {
		return p.handleFailure(ctx, bucket, key, etag, size, fmt.Errorf("flush normalized file failed: %w", err), errFile)
	}
	if err := errWriter.Flush(); err != nil {
		return p.handleFailure(ctx, bucket, key, etag, size, fmt.Errorf("flush errors file failed: %w", err), errFile)
	}

	if !p.cfg.DryRun {
		if err := p.writeToDB(ctx, records); err != nil {
			return p.handleFailure(ctx, bucket, key, etag, size, err, errFile)
		}
	}

	vaultKeys := vault.BuildKeys(key)
	if !p.cfg.DryRun {
		if err := vault.CopyObject(ctx, p.s3Client, bucket, key, p.cfg.VaultBucket, vaultKeys.Raw); err != nil {
			return p.handleFailure(ctx, bucket, key, etag, size, fmt.Errorf("vault raw copy failed: %w", err), errFile)
		}
		if err := uploadFile(ctx, p.s3Client, p.cfg.VaultBucket, vaultKeys.Normalized, normFile); err != nil {
			return p.handleFailure(ctx, bucket, key, etag, size, fmt.Errorf("vault normalized upload failed: %w", err), errFile)
		}
		if err := uploadFile(ctx, p.s3Client, p.cfg.VaultBucket, vaultKeys.Errors, errFile); err != nil {
			return p.handleFailure(ctx, bucket, key, etag, size, fmt.Errorf("vault errors upload failed: %w", err), errFile)
		}
	}

	if !p.cfg.DryRun {
		if err := p.db.InsertProcessed(ctx, bucket, key, etag, size, "PROCESSED", vaultKeys.Raw, vaultKeys.Normalized, vaultKeys.Errors, nil); err != nil {
			return p.handleFailure(ctx, bucket, key, etag, size, fmt.Errorf("insert processed_files failed: %w", err), errFile)
		}
		_ = p.db.DeleteAttempt(ctx, bucket, key, etag)
	}

	p.logger.Infof("processed %s/%s", bucket, key)
	return nil
}

func (p *Processor) parseJSONL(reader io.Reader, normWriter, errWriter *bufio.Writer) ([]model.NormalizedRecord, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var records []model.NormalizedRecord
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		record, errRecord := p.normalizeLine(line, lineNumber)
		if errRecord != nil {
			if err := writeError(errWriter, *errRecord); err != nil {
				return nil, err
			}
			continue
		}

		payload, err := json.Marshal(record)
		if err != nil {
			return nil, fmt.Errorf("marshal normalized record failed: %w", err)
		}
		if _, err := normWriter.Write(payload); err != nil {
			return nil, err
		}
		if _, err := normWriter.WriteString("\n"); err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan JSONL failed: %w", err)
	}

	return records, nil
}

func (p *Processor) normalizeLine(line string, lineNumber int) (*model.NormalizedRecord, *model.ErrorRecord) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return nil, &model.ErrorRecord{LineNumber: lineNumber, Reason: "invalid_json", RawLine: line}
	}

	address := getString(payload, "address")
	postal := getString(payload, "postal_code")
	unit := getString(payload, "unit")
	if strings.TrimSpace(address) == "" || strings.TrimSpace(postal) == "" {
		return nil, &model.ErrorRecord{LineNumber: lineNumber, Reason: "missing_identity_fields", RawLine: line}
	}

	scrapedAtStr := getString(payload, "scraped_at")
	scrapedAt, err := parseTime(scrapedAtStr)
	if err != nil {
		return nil, &model.ErrorRecord{LineNumber: lineNumber, Reason: "invalid_scraped_at", RawLine: line}
	}

	priceVal, ok := getFloat(payload, "price")
	if !ok {
		return nil, &model.ErrorRecord{LineNumber: lineNumber, Reason: "missing_price", RawLine: line}
	}

	bedsVal, _ := getInt(payload, "beds")
	bathsVal, _ := getFloatPtr(payload, "baths")
	sqftVal, _ := getInt(payload, "sqft")
	latVal, _ := getFloatPtr(payload, "lat")
	lonVal, _ := getFloatPtr(payload, "lon")

	propertyType := normalize.NormalizePropertyType(getString(payload, "property_type"))
	sourceListingID := getString(payload, "listing_id")
	url := getString(payload, "url")

	propertyKey := normalize.PropertyKey(address, postal, unit)
	snapshotHash := normalize.SnapshotHash(
		formatFloat(priceVal),
		formatIntPtr(bedsVal),
		formatFloatPtr(bathsVal),
		formatIntPtr(sqftVal),
		sourceListingID,
		propertyType,
	)

	return &model.NormalizedRecord{
		PropertyKey:     propertyKey,
		PropertyType:    propertyType,
		Address:         address,
		PostalCode:      postal,
		Unit:            normalizeStringPtr(unit),
		Price:           priceVal,
		Beds:            bedsVal,
		Baths:           bathsVal,
		Sqft:            sqftVal,
		Lat:             latVal,
		Lon:             lonVal,
		URL:             normalizeStringPtr(url),
		ScrapedAt:       scrapedAt.Format(time.RFC3339),
		SourceListingID: normalizeStringPtr(sourceListingID),
		SnapshotHash:    snapshotHash,
	}, nil
}

func (p *Processor) writeToDB(ctx context.Context, records []model.NormalizedRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := p.db.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	latestCache := map[string]string{}
	for _, record := range records {
		scrapedAt, err := time.Parse(time.RFC3339, record.ScrapedAt)
		if err != nil {
			return fmt.Errorf("invalid scraped_at in normalized record: %w", err)
		}
		if err := p.db.UpsertListing(ctx, tx, record, scrapedAt); err != nil {
			return err
		}

		latest, ok := latestCache[record.PropertyKey]
		if !ok {
			var found bool
			latest, found, err = p.db.LatestSnapshotHash(ctx, tx, record.PropertyKey)
			if err != nil {
				return err
			}
			if !found {
				latest = ""
			}
		}

		if decision.ShouldInsertHistory(latest, record.SnapshotHash) {
			if err := p.db.InsertPriceHistory(ctx, tx, record, scrapedAt); err != nil {
				return err
			}
			latestCache[record.PropertyKey] = record.SnapshotHash
		} else {
			latestCache[record.PropertyKey] = latest
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func uploadFile(ctx context.Context, client *s3.Client, bucket, key string, file *os.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return vault.PutObject(ctx, client, bucket, key, file)
}

func (p *Processor) handleFailure(ctx context.Context, bucket, key, etag string, size int64, err error, errFile *os.File) error {
	p.logger.Errorf("processing failed for %s/%s: %v", bucket, key, err)

	tempFile := errFile
	if tempFile == nil {
		created, createErr := os.CreateTemp("", "errors-failure-*.jsonl")
		if createErr != nil {
			p.logger.Warnf("failed to create failure errors file: %v", createErr)
			tempFile = nil
		} else {
			tempFile = created
			defer func() {
				_ = tempFile.Close()
				_ = os.Remove(tempFile.Name())
			}()
		}
	}

	if tempFile != nil {
		if _, writeErr := tempFile.WriteString(fmt.Sprintf("{\"line_number\":0,\"reason\":%q,\"raw_line\":%q}\n", "fatal_error", err.Error())); writeErr != nil {
			p.logger.Warnf("failed to append failure summary: %v", writeErr)
		}
		_ = tempFile.Sync()
	}

	if !p.cfg.DryRun && tempFile != nil {
		vaultKeys := vault.BuildKeys(key)
		if uploadErr := uploadFile(ctx, p.s3Client, p.cfg.VaultBucket, vaultKeys.Errors, tempFile); uploadErr != nil {
			p.logger.Warnf("failed to upload failure summary: %v", uploadErr)
		}
	}

	if etag != "" {
		if _, incErr := p.db.IncrementAttempt(ctx, bucket, key, etag, err.Error()); incErr != nil {
			p.logger.Warnf("failed to increment attempt: %v", incErr)
		}
	}

	return err
}

func writeError(writer *bufio.Writer, record model.ErrorRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := writer.Write(payload); err != nil {
		return err
	}
	if _, err := writer.WriteString("\n"); err != nil {
		return err
	}
	return nil
}

func parseTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, errors.New("empty timestamp")
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	if unixSeconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(unixSeconds, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized time format: %s", value)
}

func getString(payload map[string]any, key string) string {
	val, ok := payload[key]
	if !ok || val == nil {
		return ""
	}
	switch cast := val.(type) {
	case string:
		return strings.TrimSpace(cast)
	case fmt.Stringer:
		return strings.TrimSpace(cast.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", cast))
	}
}

func getFloat(payload map[string]any, key string) (float64, bool) {
	val, ok := payload[key]
	if !ok || val == nil {
		return 0, false
	}
	switch cast := val.(type) {
	case float64:
		return cast, true
	case int:
		return float64(cast), true
	case int64:
		return float64(cast), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(cast), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func getFloatPtr(payload map[string]any, key string) (*float64, bool) {
	value, ok := getFloat(payload, key)
	if !ok {
		return nil, false
	}
	return &value, true
}

func getInt(payload map[string]any, key string) (*int, bool) {
	val, ok := payload[key]
	if !ok || val == nil {
		return nil, false
	}
	switch cast := val.(type) {
	case float64:
		value := int(cast)
		return &value, true
	case int:
		value := cast
		return &value, true
	case int64:
		value := int(cast)
		return &value, true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(cast))
		if err != nil {
			return nil, false
		}
		return &parsed, true
	default:
		return nil, false
	}
}

func normalizeStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func formatFloatPtr(value *float64) string {
	if value == nil {
		return ""
	}
	return formatFloat(*value)
}

func formatIntPtr(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}
