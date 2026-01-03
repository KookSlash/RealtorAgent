package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var (
	multiSpaceRegex = regexp.MustCompile(`\s+`)
	nonTextRegex    = regexp.MustCompile(`[^a-z0-9 ]+`)
	nonPostalRegex  = regexp.MustCompile(`[^A-Z0-9]+`)
)

func NormalizeText(input string) string {
	value := strings.ToLower(strings.TrimSpace(input))
	value = strings.ReplaceAll(value, "\t", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = multiSpaceRegex.ReplaceAllString(value, " ")
	value = nonTextRegex.ReplaceAllString(value, "")
	value = multiSpaceRegex.ReplaceAllString(value, " ")
	return strings.TrimSpace(value)
}

func NormalizePostal(input string) string {
	value := strings.ToUpper(strings.TrimSpace(input))
	value = strings.ReplaceAll(value, " ", "")
	value = nonPostalRegex.ReplaceAllString(value, "")
	return value
}

func CanonicalIdentity(address, postalCode, unit string) string {
	return fmt.Sprintf("%s|%s|%s", NormalizeText(address), NormalizePostal(postalCode), NormalizeText(unit))
}

func PropertyKey(address, postalCode, unit string) string {
	canonical := CanonicalIdentity(address, postalCode, unit)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func SnapshotHash(price string, beds string, baths string, sqft string, sourceListingID string, propertyType string) string {
	payload := fmt.Sprintf("price=%s|beds=%s|baths=%s|sqft=%s|source_listing_id=%s|property_type=%s",
		price, beds, baths, sqft, sourceListingID, propertyType)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func NormalizePropertyType(input string) string {
	value := strings.ToUpper(strings.TrimSpace(input))
	value = strings.ReplaceAll(value, " ", "")
	switch {
	case strings.Contains(value, "HOUSE") || strings.Contains(value, "DETACHED"):
		return "HOUSE"
	case strings.Contains(value, "CONDO") || strings.Contains(value, "APARTMENT"):
		return "CONDO"
	case strings.Contains(value, "TOWN"):
		return "TOWNHOUSE"
	case strings.Contains(value, "DUPLEX"):
		return "DUPLEX"
	case strings.Contains(value, "LAND") || strings.Contains(value, "LOT"):
		return "LAND"
	default:
		return "OTHER"
	}
}
