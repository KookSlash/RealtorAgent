package strategy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/model"
	"golang.org/x/net/html"
)

var postalCodeRegex = regexp.MustCompile(`(?i)\b[abceghj-nprstvxy]\d[abceghj-nprstvxy]\s?\d[abceghj-nprstvxy]\d\b`)
var unitRegex = regexp.MustCompile(`(?i)\b(?:unit|suite|#)\s*([a-z0-9\-]+)\b`)

type ZoloHTMLStrategy struct {
	baseURL string
}

func NewZoloHTMLStrategy(baseURL string) *ZoloHTMLStrategy {
	return &ZoloHTMLStrategy{baseURL: strings.TrimSpace(baseURL)}
}

func (s *ZoloHTMLStrategy) Name() string {
	return "zolo_html"
}

func (s *ZoloHTMLStrategy) TryExtract(ctx context.Context, htmlBytes []byte, baseURL string, scrapedAt time.Time) ([]model.ListingSnapshot, bool, error) {
	_ = ctx

	normalized := normalizeZoloHTML(htmlBytes)
	doc, err := parseHTML(normalized)
	if err != nil {
		return nil, false, err
	}

	cards := findNodes(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "article" && hasClass(n, "card-listing")
	})
	if len(cards) == 0 {
		return nil, false, nil
	}

	records := make([]model.ListingSnapshot, 0, len(cards))
	for _, card := range cards {
		record, ok := s.mapCard(card, baseURL, scrapedAt)
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

func (s *ZoloHTMLStrategy) mapCard(card *html.Node, baseURL string, scrapedAt time.Time) (model.ListingSnapshot, bool) {
	listingURL := s.extractListingURL(card, baseURL)
	if listingURL == "" {
		return model.ListingSnapshot{}, false
	}

	street := extractTextByClass(card, "street")
	city := extractTextByClass(card, "city")
	provinceRaw := extractTextByClass(card, "province")
	province, postalFromProvince := splitProvincePostal(provinceRaw)
	address := formatAddress(street, city, provinceRaw)
	postal := postalFromProvince
	if postal == "" {
		postal = extractPostalCode(address)
	}
	if postal == "" {
		postal = extractPostalCode(nodeText(card))
	}

	price := extractPrice(card)
	if price <= 0 {
		return model.ListingSnapshot{}, false
	}

	beds, baths, sqft := extractListingValues(card)
	propertyType := extractPropertyType(card)
	if propertyType == "" {
		propertyType = "unknown"
	}

	listingID := extractListingID(card)
	if listingID == "" {
		listingID = deriveListingID(listingURL)
	}

	payload := buildZoloPayload(card, listingURL, street, city, province, postal, price, beds, baths, sqft, propertyType, listingID)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return model.ListingSnapshot{}, false
	}

	snapshot := model.NewListingSnapshot(scrapedAt, payloadJSON)
	snapshot.Source = model.SourceZoloCA
	snapshot.SourceListingID = listingID
	snapshot.Address = address
	snapshot.PostalCode = postal
	snapshot.Unit = extractUnit(address, listingURL)
	snapshot.City = city
	snapshot.Province = province
	snapshot.PropertyType = propertyType
	snapshot.Price = price
	snapshot.Beds = beds
	snapshot.Baths = baths
	snapshot.Sqft = sqft
	snapshot.URL = listingURL

	if strings.TrimSpace(snapshot.Address) == "" {
		return model.ListingSnapshot{}, false
	}
	if strings.TrimSpace(snapshot.URL) == "" {
		return model.ListingSnapshot{}, false
	}

	return snapshot, true
}

func (s *ZoloHTMLStrategy) extractListingURL(card *html.Node, baseURL string) string {
	link := findFirstNode(card, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "tile-overlay-link")
	})
	if link == nil {
		return ""
	}
	href, ok := getAttr(link, "href")
	if !ok || strings.TrimSpace(href) == "" {
		return ""
	}
	return resolveURLWithFallback(baseURL, s.baseURL, href)
}

func extractTextByClass(card *html.Node, class string) string {
	node := findFirstNode(card, func(n *html.Node) bool {
		return n.Type == html.ElementNode && hasClass(n, class)
	})
	if node == nil {
		return ""
	}
	return strings.TrimSpace(nodeText(node))
}

func extractPrice(card *html.Node) float64 {
	priceNode := findFirstNode(card, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "span" {
			return false
		}
		itemprop, ok := getAttr(n, "itemprop")
		if !ok {
			return false
		}
		itemprop = strings.TrimSpace(itemprop)
		return strings.EqualFold(itemprop, "price")
	})
	if priceNode == nil {
		return 0
	}
	if value, ok := getAttr(priceNode, "value"); ok {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return parsed
		}
	}
	if value, ok := getAttr(priceNode, "content"); ok {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return parsed
		}
	}
	return parseFloatFromText(nodeText(priceNode))
}

func extractListingValues(card *html.Node) (int, float64, int) {
	list := findFirstNode(card, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "ul" && hasClass(n, "card-listing--values")
	})
	if list == nil {
		return 0, 0, 0
	}

	beds := 0
	baths := 0.0
	sqft := 0

	items := findNodes(list, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "li"
	})
	for _, item := range items {
		text := strings.ToLower(nodeText(item))
		number := parseFloatFromText(text)
		switch {
		case strings.Contains(text, "bed"):
			beds = int(number)
		case strings.Contains(text, "bath"):
			baths = number
		case strings.Contains(text, "sqft") || strings.Contains(text, "sq. ft"):
			sqft = int(number)
		}
	}

	return beds, baths, sqft
}

func extractPropertyType(card *html.Node) string {
	itemType, ok := getAttr(card, "itemtype")
	if !ok || strings.TrimSpace(itemType) == "" {
		node := findFirstNode(card, func(n *html.Node) bool {
			if n.Type != html.ElementNode {
				return false
			}
			value, ok := getAttr(n, "itemtype")
			return ok && strings.TrimSpace(value) != ""
		})
		if node != nil {
			itemType, _ = getAttr(node, "itemtype")
		}
	}
	itemType = strings.TrimSpace(itemType)
	if itemType == "" {
		return ""
	}
	if parsed := propertyTypeFromItemType(itemType); parsed != "" {
		return parsed
	}
	return itemType
}

func extractListingID(card *html.Node) string {
	keys := []string{"data-mls-number", "data-mls", "data-listing-id", "data-listingkey", "data-id"}
	for _, key := range keys {
		if value, ok := getAttr(card, key); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func buildZoloPayload(card *html.Node, listingURL, street, city, province, postal string, price float64, beds int, baths float64, sqft int, propertyType, listingID string) map[string]any {
	attrs := map[string]string{}
	for _, attr := range card.Attr {
		attrs[attr.Key] = attr.Val
	}

	values := []string{}
	valuesList := findFirstNode(card, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "ul" && hasClass(n, "card-listing--values")
	})
	if valuesList != nil {
		items := findNodes(valuesList, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "li"
		})
		for _, item := range items {
			values = append(values, nodeText(item))
		}
	}

	return map[string]any{
		"url":           listingURL,
		"street":        street,
		"city":          city,
		"province":      province,
		"postal_code":   postal,
		"price":         price,
		"beds":          beds,
		"baths":         baths,
		"sqft":          sqft,
		"property_type": propertyType,
		"listing_id":    listingID,
		"values":        values,
		"attributes":    attrs,
		"text":          nodeText(card),
	}
}

func formatAddress(street, city, province string) string {
	parts := []string{}
	for _, value := range []string{street, city, province} {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, ", ")
}

func splitProvincePostal(value string) (string, string) {
	postal := extractPostalCode(value)
	if postal == "" {
		return strings.TrimSpace(value), ""
	}
	province := strings.Replace(strings.TrimSpace(value), postal, "", 1)
	province = strings.TrimSpace(province)
	return province, normalizePostal(postal)
}

func extractPostalCode(value string) string {
	match := postalCodeRegex.FindString(value)
	if match == "" {
		return ""
	}
	return normalizePostal(match)
}

func normalizePostal(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	if len(value) == 6 {
		return value[:3] + " " + value[3:]
	}
	return value
}

func parseFloatFromText(text string) float64 {
	value := strings.TrimSpace(text)
	digits := strings.Builder{}
	hasDot := false
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
			continue
		}
		if r == '.' && !hasDot {
			hasDot = true
			digits.WriteRune(r)
		}
	}
	if digits.Len() == 0 {
		return 0
	}
	parsed, err := strconv.ParseFloat(digits.String(), 64)
	if err != nil {
		return 0
	}
	return parsed
}

func propertyTypeFromItemType(itemType string) string {
	parsed, err := url.Parse(itemType)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func resolveURLWithFallback(baseURL, fallbackBase, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		return parsed.String()
	}

	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = strings.TrimSpace(fallbackBase)
	}
	if base == "" {
		return href
	}
	baseParsed, err := url.Parse(base)
	if err != nil {
		return href
	}
	return baseParsed.ResolveReference(parsed).String()
}

func deriveListingID(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return strings.TrimSpace(rawURL)
	}
	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return strings.TrimSpace(rawURL)
	}
	return path
}

func extractUnit(address, listingURL string) string {
	if match := unitRegex.FindStringSubmatch(address); len(match) > 1 {
		return match[1]
	}
	lower := strings.ToLower(listingURL)
	if strings.Contains(lower, "unit") || strings.Contains(lower, "suite") {
		if match := unitRegex.FindStringSubmatch(lower); len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

func normalizeZoloHTML(htmlBytes []byte) []byte {
	if !looksLikeViewSource(htmlBytes) {
		return htmlBytes
	}
	doc, err := parseHTML(htmlBytes)
	if err != nil {
		return htmlBytes
	}
	nodes := findNodes(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "td" && hasClass(n, "line-content")
	})
	if len(nodes) == 0 {
		return htmlBytes
	}
	var builder strings.Builder
	for _, node := range nodes {
		line := strings.TrimSpace(nodeText(node))
		if line == "" {
			continue
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	normalized := builder.String()
	if strings.Contains(normalized, "<html") || strings.Contains(normalized, "<!DOCTYPE") {
		return []byte(normalized)
	}
	return htmlBytes
}

func looksLikeViewSource(htmlBytes []byte) bool {
	if len(htmlBytes) == 0 {
		return false
	}
	return bytes.Contains(htmlBytes, []byte("line-content")) && bytes.Contains(htmlBytes, []byte("html-tag"))
}
