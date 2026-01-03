package strategy

import (
	"context"
	"strings"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/model"
	"golang.org/x/net/html"
)

type DomFallbackStrategy struct {
	mapper ListingMapper
}

func NewDomFallbackStrategy(mapper ListingMapper) *DomFallbackStrategy {
	return &DomFallbackStrategy{mapper: mapper}
}

func (s *DomFallbackStrategy) Name() string {
	return "dom_fallback"
}

func (s *DomFallbackStrategy) TryExtract(ctx context.Context, htmlBytes []byte, baseURL string, scrapedAt time.Time) ([]model.ListingSnapshot, bool, error) {
	_ = ctx

	doc, err := parseHTML(htmlBytes)
	if err != nil {
		return nil, false, err
	}

	cards := findNodes(doc, isListingCard)
	if len(cards) == 0 {
		return nil, false, nil
	}

	records := make([]model.ListingSnapshot, 0, len(cards))
	for _, card := range cards {
		raw := extractCardData(card)
		if len(raw) == 0 {
			continue
		}
		record, ok := s.mapper(raw, baseURL, scrapedAt)
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

func isListingCard(node *html.Node) bool {
	if node.Type != html.ElementNode {
		return false
	}
	if hasClass(node, "listing-card") || hasClass(node, "listing") {
		return true
	}
	if _, ok := getAttr(node, "data-listing-id"); ok {
		return true
	}
	if _, ok := getAttr(node, "data-mls-number"); ok {
		return true
	}
	if value, ok := getAttr(node, "data-testid"); ok && strings.Contains(strings.ToLower(value), "listing") {
		return true
	}
	return false
}

func extractCardData(card *html.Node) map[string]any {
	raw := map[string]any{}

	for _, attr := range card.Attr {
		switch strings.ToLower(attr.Key) {
		case "data-listing-id":
			raw["Id"] = attr.Val
		case "data-mls-number":
			raw["MlsNumber"] = attr.Val
		case "data-address", "data-address-text":
			raw["AddressText"] = attr.Val
		case "data-postal-code", "data-postal":
			raw["PostalCode"] = attr.Val
		case "data-price":
			raw["Price"] = attr.Val
		case "data-property-type":
			raw["PropertyType"] = attr.Val
		case "data-beds":
			raw["Bedrooms"] = attr.Val
		case "data-baths":
			raw["Bathrooms"] = attr.Val
		case "data-sqft":
			raw["SizeInterior"] = attr.Val
		case "data-lat":
			raw["Latitude"] = attr.Val
		case "data-lon":
			raw["Longitude"] = attr.Val
		case "data-url":
			raw["RelativeDetailsURL"] = attr.Val
		}
	}

	if _, ok := raw["AddressText"]; !ok {
		if text := findTextByClass(card, "address"); text != "" {
			raw["AddressText"] = text
		}
	}
	if _, ok := raw["PostalCode"]; !ok {
		if text := findTextByClass(card, "postal"); text != "" {
			raw["PostalCode"] = text
		}
	}
	if _, ok := raw["Price"]; !ok {
		if text := findTextByClass(card, "price"); text != "" {
			raw["Price"] = text
		}
	}
	if _, ok := raw["PropertyType"]; !ok {
		if text := findTextByClass(card, "property-type"); text != "" {
			raw["PropertyType"] = text
		}
	}
	if _, ok := raw["Bedrooms"]; !ok {
		if text := findTextByClass(card, "beds"); text != "" {
			raw["Bedrooms"] = text
		}
	}
	if _, ok := raw["Bathrooms"]; !ok {
		if text := findTextByClass(card, "baths"); text != "" {
			raw["Bathrooms"] = text
		}
	}
	if _, ok := raw["SizeInterior"]; !ok {
		if text := findTextByClass(card, "sqft"); text != "" {
			raw["SizeInterior"] = text
		}
	}
	if _, ok := raw["Latitude"]; !ok {
		if text := findTextByClass(card, "lat"); text != "" {
			raw["Latitude"] = text
		}
	}
	if _, ok := raw["Longitude"]; !ok {
		if text := findTextByClass(card, "lon"); text != "" {
			raw["Longitude"] = text
		}
	}
	if _, ok := raw["RelativeDetailsURL"]; !ok {
		if href := findFirstHref(card); href != "" {
			raw["RelativeDetailsURL"] = href
		}
	}

	return raw
}

func findTextByClass(root *html.Node, classSubstring string) string {
	node := findFirstNode(root, func(n *html.Node) bool {
		return n.Type == html.ElementNode && hasClass(n, classSubstring)
	})
	if node == nil {
		return ""
	}
	return nodeText(node)
}

func findFirstHref(root *html.Node) string {
	node := findFirstNode(root, func(n *html.Node) bool {
		if n.Type != html.ElementNode || !strings.EqualFold(n.Data, "a") {
			return false
		}
		href, ok := getAttr(n, "href")
		return ok && strings.TrimSpace(href) != ""
	})
	if node == nil {
		return ""
	}
	href, _ := getAttr(node, "href")
	return strings.TrimSpace(href)
}
