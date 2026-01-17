package message

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func TestParseSQSMessage(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		expectAction Action
		expectReason string
		expectCount  int
	}{
		{
			name:         "empty body",
			body:         "",
			expectAction: ActionDelete,
			expectReason: "empty_body",
			expectCount:  0,
		},
		{
			name:         "invalid json",
			body:         readFixture(t, "sqs/invalid.json"),
			expectAction: ActionDelete,
			expectReason: "invalid_json",
			expectCount:  0,
		},
		{
			name:         "test event",
			body:         readFixture(t, "sqs/test_event.json"),
			expectAction: ActionDelete,
			expectReason: "test_event",
			expectCount:  0,
		},
		{
			name:         "test event with service",
			body:         readFixture(t, "sqs/test_event_service.json"),
			expectAction: ActionDelete,
			expectReason: "test_event",
			expectCount:  0,
		},
		{
			name:         "no records",
			body:         readFixture(t, "sqs/no_records.json"),
			expectAction: ActionDelete,
			expectReason: "no_records",
			expectCount:  0,
		},
		{
			name:         "single record",
			body:         readFixture(t, "sqs/object_created_single.json"),
			expectAction: ActionProcess,
			expectReason: "",
			expectCount:  1,
		},
		{
			name:         "multiple records with invalid entries",
			body:         readFixture(t, "sqs/object_created_multi.json"),
			expectAction: ActionProcess,
			expectReason: "",
			expectCount:  2,
		},
		{
			name:         "no valid records",
			body:         `{"Records":[{"s3":{"bucket":{"name":""},"object":{"key":""}}}]}`,
			expectAction: ActionDelete,
			expectReason: "no_valid_records",
			expectCount:  0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseSQSMessage(tc.body)
			if result.Action != tc.expectAction {
				t.Fatalf("expected action %s, got %s", tc.expectAction, result.Action)
			}
			if result.Reason != tc.expectReason {
				t.Fatalf("expected reason %q, got %q", tc.expectReason, result.Reason)
			}
			if len(result.Records) != tc.expectCount {
				t.Fatalf("expected %d records, got %d", tc.expectCount, len(result.Records))
			}
		})
	}
}

func FuzzSQSMessageParsing(f *testing.F) {
	f.Add("{}")
	f.Add(`{"Records":[{"s3":{"bucket":{"name":"raw-bucket"},"object":{"key":"raw/realtorca/2025-01-01/run-1.jsonl"}}}]}`)
	f.Fuzz(func(t *testing.T, input string) {
		result := ParseSQSMessage(input)
		if result.Action != ActionDelete && result.Action != ActionProcess {
			t.Fatalf("unexpected action: %v", result.Action)
		}
	})
}
