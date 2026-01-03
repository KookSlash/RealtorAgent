package processor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sylvain/realtoragent/services/parser/internal/model"
	"github.com/sylvain/realtoragent/services/parser/internal/normalize"
)

func TestNormalizeLineValid(t *testing.T) {
	p := &Processor{}
	line := `{"listing_id":"L-1","address":"123 Main St","postal_code":"T2P 1A1","unit":"101","property_type":"Condo","price":"500000","beds":"2","baths":"1.5","sqft":"900","lat":"51.0","lon":"-114.0","url":"http://example.com/1","scraped_at":"2025-01-01T00:00:00Z"}`

	record, errRec := p.normalizeLine(line, 1)
	if errRec != nil {
		t.Fatalf("unexpected error: %v", errRec)
	}

	if record.PropertyType != "CONDO" {
		t.Fatalf("expected property type CONDO, got %s", record.PropertyType)
	}

	expectedKey := normalize.PropertyKey("123 Main St", "T2P 1A1", "101")
	if record.PropertyKey != expectedKey {
		t.Fatalf("unexpected property key: %s", record.PropertyKey)
	}

	expectedHash := normalize.SnapshotHash("500000", "2", "1.5", "900", "L-1", "CONDO")
	if record.SnapshotHash != expectedHash {
		t.Fatalf("unexpected snapshot hash: %s", record.SnapshotHash)
	}

	if record.Beds == nil || *record.Beds != 2 {
		t.Fatalf("expected beds=2, got %+v", record.Beds)
	}
	if record.Baths == nil || *record.Baths != 1.5 {
		t.Fatalf("expected baths=1.5, got %+v", record.Baths)
	}
	if record.Sqft == nil || *record.Sqft != 900 {
		t.Fatalf("expected sqft=900, got %+v", record.Sqft)
	}
	if record.Lat == nil || *record.Lat != 51.0 {
		t.Fatalf("expected lat=51.0, got %+v", record.Lat)
	}
	if record.Lon == nil || *record.Lon != -114.0 {
		t.Fatalf("expected lon=-114.0, got %+v", record.Lon)
	}
}

func TestNormalizeLineSnapshotHashIgnoresURL(t *testing.T) {
	p := &Processor{}
	lineA := `{"listing_id":"L-1","address":"123 Main St","postal_code":"T2P 1A1","unit":"101","property_type":"Condo","price":500000,"beds":2,"baths":1.5,"sqft":900,"url":"http://example.com/1","scraped_at":"2025-01-01T00:00:00Z"}`
	lineB := `{"listing_id":"L-1","address":"123 Main St","postal_code":"T2P 1A1","unit":"101","property_type":"Condo","price":500000,"beds":2,"baths":1.5,"sqft":900,"url":"http://example.com/2","scraped_at":"2025-01-01T00:00:00Z"}`

	recA, errA := p.normalizeLine(lineA, 1)
	if errA != nil {
		t.Fatalf("unexpected error: %v", errA)
	}
	recB, errB := p.normalizeLine(lineB, 1)
	if errB != nil {
		t.Fatalf("unexpected error: %v", errB)
	}

	if recA.SnapshotHash != recB.SnapshotHash {
		t.Fatalf("expected snapshot hash to ignore url changes")
	}
}

func TestNormalizeLineErrors(t *testing.T) {
	p := &Processor{}
	cases := []struct {
		name   string
		line   string
		reason string
	}{
		{name: "invalid json", line: "{", reason: "invalid_json"},
		{name: "missing identity", line: `{"address":"","postal_code":"","price":1,"scraped_at":"2025-01-01T00:00:00Z"}`, reason: "missing_identity_fields"},
		{name: "missing price", line: `{"address":"123 Main St","postal_code":"T2P 1A1","scraped_at":"2025-01-01T00:00:00Z"}`, reason: "missing_price"},
		{name: "invalid scraped_at", line: `{"address":"123 Main St","postal_code":"T2P 1A1","price":1,"scraped_at":"not-a-time"}`, reason: "invalid_scraped_at"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, errRec := p.normalizeLine(tc.line, 1)
			if errRec == nil {
				t.Fatalf("expected error record")
			}
			if errRec.Reason != tc.reason {
				t.Fatalf("expected reason %s, got %s", tc.reason, errRec.Reason)
			}
		})
	}
}

func TestParseJSONL(t *testing.T) {
	p := &Processor{}
	input := strings.Join([]string{
		`{"listing_id":"L-1","address":"123 Main St","postal_code":"T2P 1A1","property_type":"Condo","price":500000,"beds":2,"baths":1.5,"sqft":900,"scraped_at":"2025-01-01T00:00:00Z"}`,
		"not-json",
		`{"listing_id":"L-2","address":"456 Elm St","postal_code":"T2P 2B2","property_type":"House","price":750000,"beds":3,"baths":2,"sqft":1400,"scraped_at":"2025-01-01T00:00:00Z"}`,
	}, "\n") + "\n"

	var normBuf bytes.Buffer
	var errBuf bytes.Buffer

	normWriter := bufio.NewWriter(&normBuf)
	errWriter := bufio.NewWriter(&errBuf)

	records, err := p.parseJSONL(strings.NewReader(input), normWriter, errWriter)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if err := normWriter.Flush(); err != nil {
		t.Fatalf("flush normalized writer: %v", err)
	}
	if err := errWriter.Flush(); err != nil {
		t.Fatalf("flush error writer: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 valid records, got %d", len(records))
	}

	normLines := strings.Split(strings.TrimSpace(normBuf.String()), "\n")
	if len(normLines) != 2 {
		t.Fatalf("expected 2 normalized lines, got %d", len(normLines))
	}

	errLines := strings.Split(strings.TrimSpace(errBuf.String()), "\n")
	if len(errLines) != 1 {
		t.Fatalf("expected 1 error line, got %d", len(errLines))
	}

	var errRec model.ErrorRecord
	if err := json.Unmarshal([]byte(errLines[0]), &errRec); err != nil {
		t.Fatalf("unmarshal error record: %v", err)
	}
	if errRec.Reason != "invalid_json" {
		t.Fatalf("expected invalid_json reason, got %s", errRec.Reason)
	}
}
