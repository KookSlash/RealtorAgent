package tests

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sylvain/realtoragent/services/parser/internal/awsclient"
	"github.com/sylvain/realtoragent/services/parser/internal/config"
	"github.com/sylvain/realtoragent/services/parser/internal/db"
	"github.com/sylvain/realtoragent/services/parser/internal/normalize"
	"github.com/sylvain/realtoragent/services/parser/internal/processor"
)

type testEnv struct {
	cfg       config.Config
	s3Client  *s3.Client
	sqsClient *sqs.Client
	queueURL  string
	proc      *processor.Processor
	sqlDB     *sql.DB
	dbClient  *db.DB
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config load failed: %v", err)
	}

	ctx := context.Background()
	awsCfg, err := awsclient.NewAWSConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("aws config failed: %v", err)
	}

	s3Client := awsclient.NewS3Client(awsCfg)
	sqsClient := awsclient.NewSQSClient(awsCfg)
	queueURL := mustQueueURL(t, sqsClient, cfg.RawEventsQueue)

	dbClient, err := db.New(ctx, cfg.PostgresDSN)
	if err != nil {
		t.Fatalf("db init failed: %v", err)
	}

	proc, err := processor.New(ctx, cfg, s3Client, sqsClient, dbClient)
	if err != nil {
		t.Fatalf("processor init failed: %v", err)
	}

	sqlDB, err := sql.Open("pgx", cfg.PostgresDSN)
	if err != nil {
		t.Fatalf("sql open failed: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
		_ = dbClient.Close()
	})

	return &testEnv{
		cfg:       cfg,
		s3Client:  s3Client,
		sqsClient: sqsClient,
		queueURL:  queueURL,
		proc:      proc,
		sqlDB:     sqlDB,
		dbClient:  dbClient,
	}
}

func mustQueueURL(t *testing.T, client *sqs.Client, name string) string {
	ctx := context.Background()
	out, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(name)})
	if err != nil {
		t.Fatalf("queue url lookup failed: %v", err)
	}
	return aws.ToString(out.QueueUrl)
}

func drainQueue(t *testing.T, env *testEnv) {
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		out, err := env.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(env.queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     1,
		})
		if err != nil {
			t.Fatalf("receive message failed: %v", err)
		}
		if len(out.Messages) == 0 {
			return
		}
		for _, msg := range out.Messages {
			_, err := env.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(env.queueURL),
				ReceiptHandle: msg.ReceiptHandle,
			})
			if err != nil {
				t.Fatalf("delete message failed: %v", err)
			}
		}
	}
}

func waitForMessages(t *testing.T, env *testEnv) {
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		out, err := env.sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl: aws.String(env.queueURL),
			AttributeNames: []types.QueueAttributeName{
				types.QueueAttributeNameApproximateNumberOfMessages,
			},
		})
		if err != nil {
			t.Fatalf("get queue attributes failed: %v", err)
		}
		countStr := out.Attributes[string(types.QueueAttributeNameApproximateNumberOfMessages)]
		if countStr != "" && countStr != "0" {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func putObject(t *testing.T, env *testEnv, key string, body string) {
	ctx := context.Background()
	_, err := env.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(env.cfg.RawBucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(body),
	})
	if err != nil {
		t.Fatalf("put object failed: %v", err)
	}
}

func sendTestEvent(t *testing.T, env *testEnv) {
	ctx := context.Background()
	_, err := env.sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(env.queueURL),
		MessageBody: aws.String(`{"Event":"s3:TestEvent"}`),
	})
	if err != nil {
		t.Fatalf("send test event failed: %v", err)
	}
}

func sendS3Event(t *testing.T, env *testEnv, bucket, key string) {
	ctx := context.Background()
	body := fmt.Sprintf(`{"Records":[{"s3":{"bucket":{"name":"%s"},"object":{"key":"%s"}}}]}`, bucket, key)
	_, err := env.sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(env.queueURL),
		MessageBody: aws.String(body),
	})
	if err != nil {
		t.Fatalf("send s3 event failed: %v", err)
	}
}

func runProcessorOnce(t *testing.T, env *testEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return env.proc.ProcessOnce(ctx)
}

func countTable(t *testing.T, env *testEnv, table string) int {
	row := env.sqlDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table))
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count %s failed: %v", table, err)
	}
	return count
}

func countProcessedFile(t *testing.T, env *testEnv, bucket, key, etag string) int {
	row := env.sqlDB.QueryRow("SELECT COUNT(*) FROM processed_files WHERE s3_bucket=$1 AND s3_key=$2 AND etag=$3 AND status='PROCESSED'", bucket, key, etag)
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("processed_files count failed: %v", err)
	}
	return count
}

func countListingsByKeys(t *testing.T, env *testEnv, keys []string) int {
	return countByKeys(t, env, "listings", keys)
}

func countHistoryByKeys(t *testing.T, env *testEnv, keys []string) int {
	return countByKeys(t, env, "price_history", keys)
}

func countByKeys(t *testing.T, env *testEnv, table string, keys []string) int {
	if len(keys) == 0 {
		return 0
	}

	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))
	for i, key := range keys {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = key
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE property_key IN (%s)", table, strings.Join(placeholders, ","))
	row := env.sqlDB.QueryRow(query, args...)
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count %s failed: %v", table, err)
	}
	return count
}

func countHistoryForKey(t *testing.T, env *testEnv, key string) int {
	row := env.sqlDB.QueryRow("SELECT COUNT(*) FROM price_history WHERE property_key=$1", key)
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("price_history count failed: %v", err)
	}
	return count
}

func getETag(t *testing.T, env *testEnv, bucket, key string) string {
	ctx := context.Background()
	out, err := env.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("head object failed: %v", err)
	}
	return strings.Trim(aws.ToString(out.ETag), "\"")
}

func getObjectLines(t *testing.T, env *testEnv, bucket, key string) []string {
	ctx := context.Background()
	out, err := env.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("get object failed: %v", err)
	}
	defer out.Body.Close()

	lines := []string{}
	scanner := bufio.NewScanner(out.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read object failed: %v", err)
	}
	return lines
}

func TestIgnoresTestEvent(t *testing.T) {
	env := newTestEnv(t)
	drainQueue(t, env)

	beforeProcessed := countTable(t, env, "processed_files")
	beforeListings := countTable(t, env, "listings")
	beforeHistory := countTable(t, env, "price_history")
	beforeAttempts := countTable(t, env, "processing_attempts")

	sendTestEvent(t, env)
	waitForMessages(t, env)

	if err := runProcessorOnce(t, env); err != nil {
		t.Fatalf("processor failed: %v", err)
	}

	afterProcessed := countTable(t, env, "processed_files")
	afterListings := countTable(t, env, "listings")
	afterHistory := countTable(t, env, "price_history")
	afterAttempts := countTable(t, env, "processing_attempts")

	if beforeProcessed != afterProcessed || beforeListings != afterListings || beforeHistory != afterHistory || beforeAttempts != afterAttempts {
		t.Fatalf("expected no DB changes for TestEvent")
	}
}

func TestProcessesAndArchives(t *testing.T) {
	env := newTestEnv(t)
	drainQueue(t, env)

	dateStr := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf("raw/realtorca/%s/run-test-t2-%d.jsonl", dateStr, time.Now().UnixNano())

	valid1 := `{"listing_id":"L-1","address":"123 Main St","postal_code":"T2P 1A1","unit":"101","property_type":"Condo","price":500000,"beds":2,"baths":1.5,"sqft":900,"lat":51.0447,"lon":-114.0719,"url":"http://example.com/1","scraped_at":"2025-01-01T00:00:00Z"}`
	valid2 := `{"listing_id":"L-2","address":"456 Elm St","postal_code":"T2P 2B2","property_type":"House","price":750000,"beds":3,"baths":2,"sqft":1400,"lat":51.05,"lon":-114.07,"url":"http://example.com/2","scraped_at":"2025-01-01T00:00:00Z"}`
	invalid := `not-json`
	content := strings.Join([]string{valid1, valid2, invalid}, "\n") + "\n"

	putObject(t, env, key, content)
	waitForMessages(t, env)

	if err := runProcessorOnce(t, env); err != nil {
		t.Fatalf("processor failed: %v", err)
	}

	etag := getETag(t, env, env.cfg.RawBucket, key)
	if count := countProcessedFile(t, env, env.cfg.RawBucket, key, etag); count != 1 {
		t.Fatalf("expected processed_files row, got %d", count)
	}

	key1 := normalize.PropertyKey("123 Main St", "T2P 1A1", "101")
	key2 := normalize.PropertyKey("456 Elm St", "T2P 2B2", "")
	propertyKeys := []string{key1, key2}

	if count := countListingsByKeys(t, env, propertyKeys); count != 2 {
		t.Fatalf("expected 2 listings, got %d", count)
	}
	if count := countHistoryByKeys(t, env, propertyKeys); count != 2 {
		t.Fatalf("expected 2 price_history rows, got %d", count)
	}

	vaultRaw := fmt.Sprintf("vault/raw/%s", key)
	vaultNorm := fmt.Sprintf("vault/normalized/%s.normalized.jsonl", key)
	vaultErr := fmt.Sprintf("vault/errors/%s.errors.jsonl", key)

	if _, err := env.s3Client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(env.cfg.VaultBucket),
		Key:    aws.String(vaultRaw),
	}); err != nil {
		t.Fatalf("vault raw missing: %v", err)
	}
	if _, err := env.s3Client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(env.cfg.VaultBucket),
		Key:    aws.String(vaultNorm),
	}); err != nil {
		t.Fatalf("vault normalized missing: %v", err)
	}
	if _, err := env.s3Client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(env.cfg.VaultBucket),
		Key:    aws.String(vaultErr),
	}); err != nil {
		t.Fatalf("vault errors missing: %v", err)
	}

	normLines := getObjectLines(t, env, env.cfg.VaultBucket, vaultNorm)
	if len(normLines) != 2 {
		t.Fatalf("expected 2 normalized lines, got %d", len(normLines))
	}
	errLines := getObjectLines(t, env, env.cfg.VaultBucket, vaultErr)
	if len(errLines) == 0 {
		t.Fatalf("expected errors lines")
	}
	joined := strings.Join(errLines, "\n")
	if !strings.Contains(joined, "invalid_json") || !strings.Contains(joined, "not-json") {
		t.Fatalf("expected errors file to contain invalid line and reason")
	}
}

func TestIdempotency(t *testing.T) {
	env := newTestEnv(t)
	drainQueue(t, env)

	dateStr := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf("raw/realtorca/%s/run-test-t3-%d.jsonl", dateStr, time.Now().UnixNano())

	valid := `{"listing_id":"L-3","address":"789 Pine St","postal_code":"T2P 3C3","property_type":"House","price":600000,"beds":4,"baths":2,"sqft":1600,"lat":51.06,"lon":-114.08,"url":"http://example.com/3","scraped_at":"2025-01-02T00:00:00Z"}`
	content := valid + "\n"

	putObject(t, env, key, content)
	waitForMessages(t, env)

	if err := runProcessorOnce(t, env); err != nil {
		t.Fatalf("processor failed: %v", err)
	}

	etag := getETag(t, env, env.cfg.RawBucket, key)
	processedCount := countProcessedFile(t, env, env.cfg.RawBucket, key, etag)
	if processedCount != 1 {
		t.Fatalf("expected processed_files row, got %d", processedCount)
	}

	propertyKey := normalize.PropertyKey("789 Pine St", "T2P 3C3", "")
	listingsCount := countListingsByKeys(t, env, []string{propertyKey})
	historyCount := countHistoryByKeys(t, env, []string{propertyKey})

	sendS3Event(t, env, env.cfg.RawBucket, key)
	waitForMessages(t, env)
	if err := runProcessorOnce(t, env); err != nil {
		t.Fatalf("processor failed on rerun: %v", err)
	}

	processedAfter := countProcessedFile(t, env, env.cfg.RawBucket, key, etag)
	listingsAfter := countListingsByKeys(t, env, []string{propertyKey})
	historyAfter := countHistoryByKeys(t, env, []string{propertyKey})

	if processedAfter != processedCount || listingsAfter != listingsCount || historyAfter != historyCount {
		t.Fatalf("idempotency violated")
	}
}

func TestEventOnlyHistory(t *testing.T) {
	env := newTestEnv(t)
	drainQueue(t, env)

	dateStr := time.Now().UTC().Format("2006-01-02")
	key1 := fmt.Sprintf("raw/realtorca/%s/run-test-t4a-%d.jsonl", dateStr, time.Now().UnixNano())
	key2 := fmt.Sprintf("raw/realtorca/%s/run-test-t4b-%d.jsonl", dateStr, time.Now().UnixNano())
	key3 := fmt.Sprintf("raw/realtorca/%s/run-test-t4c-%d.jsonl", dateStr, time.Now().UnixNano())

	uniqueAddress := fmt.Sprintf("1000 Maple Ave %d", time.Now().UnixNano())
	base := map[string]any{
		"listing_id":    "L-4",
		"address":       uniqueAddress,
		"postal_code":   "T2P 4D4",
		"property_type": "House",
		"beds":          3,
		"baths":         2,
		"sqft":          1500,
		"lat":           51.07,
		"lon":           -114.09,
		"url":           "http://example.com/4",
	}

	line1 := withPriceAndTime(t, base, 550000, "2025-01-03T00:00:00Z")
	line2 := withPriceAndTime(t, base, 550000, "2025-01-04T00:00:00Z")
	line3 := withPriceAndTime(t, base, 575000, "2025-01-05T00:00:00Z")

	putObject(t, env, key1, line1+"\n")
	waitForMessages(t, env)
	if err := runProcessorOnce(t, env); err != nil {
		t.Fatalf("processor failed: %v", err)
	}

	propertyKey := normalize.PropertyKey(uniqueAddress, "T2P 4D4", "")
	count1 := countHistoryForKey(t, env, propertyKey)
	if count1 != 1 {
		t.Fatalf("expected 1 history row, got %d", count1)
	}

	putObject(t, env, key2, line2+"\n")
	waitForMessages(t, env)
	if err := runProcessorOnce(t, env); err != nil {
		t.Fatalf("processor failed: %v", err)
	}

	count2 := countHistoryForKey(t, env, propertyKey)
	if count2 != 1 {
		t.Fatalf("expected no new history row, got %d", count2)
	}

	putObject(t, env, key3, line3+"\n")
	waitForMessages(t, env)
	if err := runProcessorOnce(t, env); err != nil {
		t.Fatalf("processor failed: %v", err)
	}

	count3 := countHistoryForKey(t, env, propertyKey)
	if count3 != 2 {
		t.Fatalf("expected new history row, got %d", count3)
	}
}

func TestRetryLimit(t *testing.T) {
	env := newTestEnv(t)
	drainQueue(t, env)

	dateStr := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf("raw/realtorca/%s/run-test-t5-%d.jsonl", dateStr, time.Now().UnixNano())
	valid := `{"listing_id":"L-5","address":"2000 Spruce Rd","postal_code":"T2P 5E5","property_type":"House","price":650000,"beds":3,"baths":2,"sqft":1550,"lat":51.08,"lon":-114.1,"url":"http://example.com/5","scraped_at":"2025-01-06T00:00:00Z"}`

	putObject(t, env, key, valid+"\n")
	waitForMessages(t, env)

	env.cfg.MaxRetries = 2
	env.cfg.FailOnKey = key
	env.cfg.SQSVisibilityTimeout = 1

	proc, err := processor.New(context.Background(), env.cfg, env.s3Client, env.sqsClient, env.dbClient)
	if err != nil {
		t.Fatalf("processor init failed: %v", err)
	}
	env.proc = proc

	etag := getETag(t, env, env.cfg.RawBucket, key)

	for i := 1; i <= env.cfg.MaxRetries; i++ {
		err = runProcessorOnce(t, env)
		if err == nil {
			t.Fatalf("expected failure on attempt %d", i)
		}
		attempts := attemptCount(t, env, env.cfg.RawBucket, key, etag)
		if attempts != i {
			t.Fatalf("expected attempt_count %d, got %d", i, attempts)
		}
		if count := countProcessedFile(t, env, env.cfg.RawBucket, key, etag); count != 0 {
			t.Fatalf("did not expect processed_files row")
		}
		time.Sleep(1500 * time.Millisecond)
	}

	err = runProcessorOnce(t, env)
	if err != nil {
		t.Fatalf("expected no error after max retries, got %v", err)
	}
	attempts := attemptCount(t, env, env.cfg.RawBucket, key, etag)
	if attempts != env.cfg.MaxRetries {
		t.Fatalf("expected attempt_count to remain at max, got %d", attempts)
	}

	if !queueEmpty(t, env) {
		t.Fatalf("expected queue to be empty after max retries")
	}
}

func attemptCount(t *testing.T, env *testEnv, bucket, key, etag string) int {
	row := env.sqlDB.QueryRow("SELECT attempt_count FROM processing_attempts WHERE s3_bucket=$1 AND s3_key=$2 AND etag=$3", bucket, key, etag)
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("processing_attempts lookup failed: %v", err)
	}
	return count
}

func queueEmpty(t *testing.T, env *testEnv) bool {
	ctx := context.Background()
	out, err := env.sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(env.queueURL),
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameApproximateNumberOfMessages,
		},
	})
	if err != nil {
		t.Fatalf("get queue attributes failed: %v", err)
	}
	countStr := out.Attributes[string(types.QueueAttributeNameApproximateNumberOfMessages)]
	return countStr == "0" || countStr == ""
}

func withPriceAndTime(t *testing.T, base map[string]any, price int, scrapedAt string) string {
	payload := map[string]any{}
	for k, v := range base {
		payload[k] = v
	}
	payload["price"] = price
	payload["scraped_at"] = scrapedAt

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	return string(data)
}
