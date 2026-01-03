package write

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/model"
)

func TestJSONLWriter(t *testing.T) {
	var buf bytes.Buffer
	writer := NewJSONLWriter(&buf)

	payload := json.RawMessage(`{"raw":true}`)
	first := model.NewListingSnapshot(time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC), payload)
	first.Address = "123 Main St"
	first.PostalCode = "T2P 1A1"
	first.PropertyType = "Condo"
	first.Price = 500000
	first.URL = "https://example.com/listing/1"

	second := model.NewListingSnapshot(time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC), payload)
	second.Address = "456 Elm St"
	second.PostalCode = "T2P 2B2"
	second.PropertyType = "House"
	second.Price = 750000
	second.URL = "https://example.com/listing/2"

	if err := writer.Write(first); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if err := writer.Write(second); err != nil {
		t.Fatalf("write second: %v", err)
	}

	content := buf.String()
	if !strings.HasSuffix(content, "\n") {
		t.Fatalf("expected trailing newline")
	}

	lines := []string{}
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSuffix(content, "\n")))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	for _, line := range lines {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("invalid json line: %v", err)
		}
		if decoded["address"] == "" || decoded["postal_code"] == "" {
			t.Fatalf("missing required fields in line: %s", line)
		}
	}
}
