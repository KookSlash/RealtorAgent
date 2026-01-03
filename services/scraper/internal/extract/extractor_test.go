package extract

import (
	"context"
	"testing"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/strategy"
)

func TestExtractorPrefersJSONStrategy(t *testing.T) {
	html := []byte(`
		<html>
			<body>
				<script id="SEOLandingPageInitialResponse" type="application/json">
					{"Results":[{"Id":"J-1","Property":{"Price":"$400,000","Type":"Condo","Address":{"AddressText":"1 Test St|Calgary, Alberta T2P 1A1","PostalCode":"T2P 1A1"}},"RelativeDetailsURL":"/listing/1"}]}
				</script>
				<div class="listing-card" data-listing-id="D-1">
					<a href="/listing/2">View</a>
					<span class="address">2 Test St</span>
					<span class="postal">T2P 2B2</span>
					<span class="price">$500,000</span>
					<span class="property-type">House</span>
				</div>
			</body>
		</html>
	`)

	jsonStrategy := strategy.NewJsonEmbeddedStrategy(mapListingSnapshot)
	domStrategy := strategy.NewDomFallbackStrategy(mapListingSnapshot)

	scrapedAt := time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)

	_, domMatched, err := domStrategy.TryExtract(context.Background(), html, "https://www.realtor.ca/ab/calgary/real-estate", scrapedAt)
	if err != nil {
		t.Fatalf("dom extract error: %v", err)
	}
	if !domMatched {
		t.Fatalf("expected dom strategy to match test html")
	}

	extractor := NewExtractor(jsonStrategy, domStrategy)
	records, used, err := extractor.Extract(context.Background(), html, "https://www.realtor.ca/ab/calgary/real-estate", scrapedAt)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if used != jsonStrategy.Name() {
		t.Fatalf("expected json strategy, got %s", used)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].SourceListingID != "J-1" {
		t.Fatalf("expected record from json strategy, got %s", records[0].SourceListingID)
	}
}
