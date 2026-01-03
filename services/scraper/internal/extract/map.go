package extract

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/model"
)

var postalCodePattern = regexp.MustCompile(`(?i)\b[abceghj-nprstvxy]\d[abceghj-nprstvxy]\s?\d[abceghj-nprstvxy]\d\b`)
var unitPattern = regexp.MustCompile(`(?i)\b(?:unit|#)\s*([a-z0-9\-]+)\b`)
var numberPattern = regexp.MustCompile(`\d+(?:\.\d+)?`)

func mapListingSnapshot(raw map[string]any, baseURL string, scrapedAt time.Time) (model.ListingSnapshot, bool) {
	payload, err := json.Marshal(raw)
	if err != nil {
		return model.ListingSnapshot{}, false
	}

	snapshot := model.NewListingSnapshot(scrapedAt, payload)
	snapshot.SourceListingID = firstString(raw,
		[]string{"Id"},
		[]string{"ListingId"},
		[]string{"MlsNumber"},
		[]string{"ListingID"},
		[]string{"source_listing_id"},
	)

	addressText := firstString(raw,
		[]string{"Property", "Address", "AddressText"},
		[]string{"Property", "Address", "StreetAddress"},
		[]string{"Property", "Address", "AddressLine1"},
		[]string{"Property", "AddressText"},
		[]string{"AddressText"},
		[]string{"Address"},
		[]string{"address"},
	)

	address, cityFromText, provinceFromText, postalFromText := parseAddressText(addressText)
	if address == "" {
		address = strings.TrimSpace(addressText)
	}
	snapshot.Address = address

	postal := firstString(raw,
		[]string{"Property", "Address", "PostalCode"},
		[]string{"Property", "PostalCode"},
		[]string{"PostalCode"},
		[]string{"postal_code"},
	)
	if postal == "" {
		postal = postalFromText
	}
	snapshot.PostalCode = normalizePostalCode(postal)

	snapshot.Unit = firstString(raw,
		[]string{"Property", "Address", "UnitNumber"},
		[]string{"Property", "Address", "Unit"},
		[]string{"UnitNumber"},
		[]string{"Unit"},
		[]string{"unit"},
	)
	if snapshot.Unit == "" {
		snapshot.Unit = extractUnit(addressText)
	}

	snapshot.City = firstString(raw,
		[]string{"Property", "Address", "City"},
		[]string{"Property", "City"},
		[]string{"City"},
	)
	if snapshot.City == "" {
		snapshot.City = cityFromText
	}

	snapshot.Province = firstString(raw,
		[]string{"Property", "Address", "Province"},
		[]string{"Property", "Province"},
		[]string{"Province"},
	)
	if snapshot.Province == "" {
		snapshot.Province = provinceFromText
	}

	snapshot.PropertyType = firstString(raw,
		[]string{"Property", "Type"},
		[]string{"Property", "PropertyType"},
		[]string{"PropertyType"},
		[]string{"property_type"},
	)

	snapshot.Price = firstFloat(raw,
		[]string{"Property", "Price"},
		[]string{"Property", "PriceUnformattedValue"},
		[]string{"Price"},
		[]string{"price"},
	)

	snapshot.Beds = firstInt(raw,
		[]string{"Building", "Bedrooms"},
		[]string{"Building", "BedroomsTotal"},
		[]string{"Bedrooms"},
		[]string{"Beds"},
		[]string{"beds"},
	)

	snapshot.Baths = firstFloat(raw,
		[]string{"Building", "BathroomTotal"},
		[]string{"Building", "Bathrooms"},
		[]string{"BathroomTotal"},
		[]string{"Bathrooms"},
		[]string{"Baths"},
		[]string{"baths"},
	)

	snapshot.Sqft = firstInt(raw,
		[]string{"Building", "SizeInterior"},
		[]string{"SizeInterior"},
		[]string{"Sqft"},
		[]string{"sqft"},
	)

	snapshot.Lat = firstFloat(raw,
		[]string{"Property", "Address", "Latitude"},
		[]string{"Latitude"},
		[]string{"lat"},
	)
	snapshot.Lon = firstFloat(raw,
		[]string{"Property", "Address", "Longitude"},
		[]string{"Longitude"},
		[]string{"lon"},
	)

	rawURL := firstString(raw,
		[]string{"RelativeDetailsURL"},
		[]string{"RelativeDetailsUrl"},
		[]string{"URL"},
		[]string{"Url"},
		[]string{"url"},
	)
	snapshot.URL = resolveURL(baseURL, rawURL)

	if !hasRequiredFields(snapshot) {
		return model.ListingSnapshot{}, false
	}

	return snapshot, true
}

func hasRequiredFields(snapshot model.ListingSnapshot) bool {
	if strings.TrimSpace(snapshot.Address) == "" {
		return false
	}
	if strings.TrimSpace(snapshot.PostalCode) == "" {
		return false
	}
	if strings.TrimSpace(snapshot.PropertyType) == "" {
		return false
	}
	if snapshot.Price <= 0 {
		return false
	}
	if strings.TrimSpace(snapshot.URL) == "" {
		return false
	}
	return true
}

func resolveURL(baseURL, href string) string {
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
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	return base.ResolveReference(parsed).String()
}

func firstString(raw map[string]any, paths ...[]string) string {
	for _, path := range paths {
		value, ok := lookup(raw, path...)
		if !ok {
			continue
		}
		if result, ok := stringFromAny(value); ok {
			return result
		}
	}
	return ""
}

func firstFloat(raw map[string]any, paths ...[]string) float64 {
	for _, path := range paths {
		value, ok := lookup(raw, path...)
		if !ok {
			continue
		}
		if result, ok := floatFromAny(value); ok {
			return result
		}
	}
	return 0
}

func firstInt(raw map[string]any, paths ...[]string) int {
	for _, path := range paths {
		value, ok := lookup(raw, path...)
		if !ok {
			continue
		}
		if result, ok := intFromAny(value); ok {
			return result
		}
	}
	return 0
}

func lookup(raw map[string]any, path ...string) (any, bool) {
	current := any(raw)
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok := obj[key]
		if !ok {
			return nil, false
		}
		current = value
	}
	return current, true
}

func stringFromAny(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	case json.Number:
		return typed.String(), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64), true
	case int:
		return strconv.Itoa(typed), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	default:
		return "", false
	}
}

func floatFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	case string:
		return parseNumber(typed)
	default:
		return 0, false
	}
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	case string:
		number, ok := parseNumber(typed)
		if !ok {
			return 0, false
		}
		return int(number), true
	default:
		return 0, false
	}
}

func parseNumber(value string) (float64, bool) {
	clean := strings.ReplaceAll(value, ",", "")
	match := numberPattern.FindString(clean)
	if match == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseAddressText(value string) (address, city, province, postal string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", "", ""
	}
	if strings.Contains(trimmed, "|") {
		parts := strings.SplitN(trimmed, "|", 2)
		address = strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		postal = extractPostalCode(right)
		city, province = parseCityProvince(right)
		return address, city, province, postal
	}
	postal = extractPostalCode(trimmed)
	return trimmed, "", "", postal
}

func parseCityProvince(text string) (string, string) {
	parts := strings.Split(text, ",")
	if len(parts) == 0 {
		return "", ""
	}
	city := strings.TrimSpace(parts[0])
	province := ""
	if len(parts) > 1 {
		province = strings.TrimSpace(parts[1])
	}
	return city, normalizeProvince(province)
}

func normalizeProvince(value string) string {
	if value == "" {
		return ""
	}
	upper := strings.ToUpper(strings.TrimSpace(value))
	switch upper {
	case "ALBERTA":
		return "AB"
	case "BRITISH COLUMBIA":
		return "BC"
	case "MANITOBA":
		return "MB"
	case "NEW BRUNSWICK":
		return "NB"
	case "NEWFOUNDLAND AND LABRADOR":
		return "NL"
	case "NOVA SCOTIA":
		return "NS"
	case "NORTHWEST TERRITORIES":
		return "NT"
	case "NUNAVUT":
		return "NU"
	case "ONTARIO":
		return "ON"
	case "PRINCE EDWARD ISLAND":
		return "PE"
	case "QUEBEC":
		return "QC"
	case "SASKATCHEWAN":
		return "SK"
	case "YUKON":
		return "YT"
	default:
		if len(upper) == 2 {
			return upper
		}
		return value
	}
}

func normalizePostalCode(value string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	if len(trimmed) == 6 {
		return trimmed[:3] + " " + trimmed[3:]
	}
	return trimmed
}

func extractPostalCode(text string) string {
	match := postalCodePattern.FindString(text)
	if match == "" {
		return ""
	}
	return normalizePostalCode(match)
}

func extractUnit(text string) string {
	match := unitPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}
