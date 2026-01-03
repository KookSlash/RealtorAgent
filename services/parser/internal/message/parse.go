package message

import (
	"encoding/json"
	"strings"
)

type Action string

const (
	ActionDelete  Action = "delete"
	ActionProcess Action = "process"
)

type S3Record struct {
	Bucket string
	Key    string
}

type ParseResult struct {
	Action  Action
	Reason  string
	Records []S3Record
}

type s3Event struct {
	Records []s3EventRecord `json:"Records"`
	Event   string          `json:"Event"`
}

type s3EventRecord struct {
	S3 s3Entity `json:"s3"`
}

type s3Entity struct {
	Bucket s3Bucket `json:"bucket"`
	Object s3Object `json:"object"`
}

type s3Bucket struct {
	Name string `json:"name"`
}

type s3Object struct {
	Key string `json:"key"`
}

func ParseSQSMessage(body string) ParseResult {
	if strings.TrimSpace(body) == "" {
		return ParseResult{Action: ActionDelete, Reason: "empty_body"}
	}

	var event s3Event
	if err := json.Unmarshal([]byte(body), &event); err != nil {
		return ParseResult{Action: ActionDelete, Reason: "invalid_json"}
	}

	if event.Event == "s3:TestEvent" && len(event.Records) == 0 {
		return ParseResult{Action: ActionDelete, Reason: "test_event"}
	}

	if len(event.Records) == 0 {
		return ParseResult{Action: ActionDelete, Reason: "no_records"}
	}

	records := make([]S3Record, 0, len(event.Records))
	for _, record := range event.Records {
		bucket := strings.TrimSpace(record.S3.Bucket.Name)
		key := strings.TrimSpace(record.S3.Object.Key)
		if bucket == "" || key == "" {
			continue
		}
		records = append(records, S3Record{Bucket: bucket, Key: key})
	}

	if len(records) == 0 {
		return ParseResult{Action: ActionDelete, Reason: "no_valid_records"}
	}

	return ParseResult{Action: ActionProcess, Records: records}
}
