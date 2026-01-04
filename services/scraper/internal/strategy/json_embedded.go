package strategy

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/model"
	"golang.org/x/net/html"
)

const seoJSONScriptID = "SEOLandingPageInitialResponse"

type JsonEmbeddedStrategy struct {
	mapper ListingMapper
}

func NewJsonEmbeddedStrategy(mapper ListingMapper) *JsonEmbeddedStrategy {
	return &JsonEmbeddedStrategy{mapper: mapper}
}

func (s *JsonEmbeddedStrategy) Name() string {
	return "json_embedded"
}

func (s *JsonEmbeddedStrategy) TryExtract(ctx context.Context, html []byte, baseURL string, scrapedAt time.Time) ([]model.ListingSnapshot, bool, error) {
	_ = ctx

	payload, ok, err := extractEmbeddedJSON(html)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return s.extractFromPayload(payload, baseURL, scrapedAt)
}

func (s *JsonEmbeddedStrategy) TryExtractPayload(ctx context.Context, payload []byte, baseURL string, scrapedAt time.Time) ([]model.ListingSnapshot, bool, error) {
	_ = ctx
	if len(payload) == 0 {
		return nil, false, nil
	}
	return s.extractFromPayload(payload, baseURL, scrapedAt)
}

func extractEmbeddedJSON(htmlBytes []byte) ([]byte, bool, error) {
	doc, err := parseHTML(htmlBytes)
	if err != nil {
		return nil, false, err
	}

	script := findFirstNode(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || !strings.EqualFold(n.Data, "script") {
			return false
		}
		id, ok := getAttr(n, "id")
		return ok && id == seoJSONScriptID
	})
	if script == nil {
		return nil, false, nil
	}

	text := strings.TrimSpace(nodeText(script))
	if text == "" {
		return nil, false, nil
	}
	return []byte(text), true, nil
}

func (s *JsonEmbeddedStrategy) extractFromPayload(payload []byte, baseURL string, scrapedAt time.Time) ([]model.ListingSnapshot, bool, error) {
	listings, err := extractListingMapsFromJSON(payload)
	if err != nil {
		return nil, false, err
	}
	if len(listings) == 0 {
		return nil, false, nil
	}

	records := make([]model.ListingSnapshot, 0, len(listings))
	for _, listing := range listings {
		record, ok := s.mapper(listing, baseURL, scrapedAt)
		if !ok {
			continue
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		return nil, false, nil
	}

	return records, true, nil
}

func extractListingMapsFromJSON(payload []byte) ([]map[string]any, error) {
	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	return findListingObjects(decoded), nil
}

func findListingObjects(data any) []map[string]any {
	var listings []map[string]any

	switch value := data.(type) {
	case []any:
		for _, item := range value {
			listings = append(listings, findListingObjects(item)...)
		}
	case map[string]any:
		for _, key := range []string{"Results", "Listings", "properties", "Properties"} {
			if raw, ok := value[key]; ok {
				if arr, ok := raw.([]any); ok {
					listings = append(listings, filterListings(arr)...)
				}
			}
		}
		for _, nested := range value {
			listings = append(listings, findListingObjects(nested)...)
		}
	}

	return listings
}

func filterListings(items []any) []map[string]any {
	listings := make([]map[string]any, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if looksLikeListing(entry) {
			listings = append(listings, entry)
		}
	}
	return listings
}

func looksLikeListing(data map[string]any) bool {
	score := 0
	if hasAddress(data) {
		score++
	}
	if hasPrice(data) {
		score++
	}
	if hasURL(data) {
		score++
	}
	if hasListingID(data) {
		score++
	}
	return score >= 2
}

func hasAddress(data map[string]any) bool {
	if value, ok := data["AddressText"]; ok && value != "" {
		return true
	}
	if value, ok := data["Address"]; ok && value != "" {
		return true
	}
	property, ok := data["Property"].(map[string]any)
	if !ok {
		return false
	}
	if value, ok := property["AddressText"]; ok && value != "" {
		return true
	}
	address, ok := property["Address"].(map[string]any)
	if !ok {
		return false
	}
	if value, ok := address["AddressText"]; ok && value != "" {
		return true
	}
	if value, ok := address["StreetAddress"]; ok && value != "" {
		return true
	}
	return false
}

func hasPrice(data map[string]any) bool {
	if _, ok := data["Price"]; ok {
		return true
	}
	property, ok := data["Property"].(map[string]any)
	if !ok {
		return false
	}
	if _, ok := property["Price"]; ok {
		return true
	}
	if _, ok := property["PriceUnformattedValue"]; ok {
		return true
	}
	return false
}

func hasURL(data map[string]any) bool {
	if _, ok := data["RelativeDetailsURL"]; ok {
		return true
	}
	if _, ok := data["RelativeDetailsUrl"]; ok {
		return true
	}
	if _, ok := data["URL"]; ok {
		return true
	}
	if _, ok := data["Url"]; ok {
		return true
	}
	return false
}

func hasListingID(data map[string]any) bool {
	if _, ok := data["Id"]; ok {
		return true
	}
	if _, ok := data["ListingId"]; ok {
		return true
	}
	if _, ok := data["MlsNumber"]; ok {
		return true
	}
	return false
}
