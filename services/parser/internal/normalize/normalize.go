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
	unitPrefixRegex = regexp.MustCompile(`^\s*(\d+)\s*-\s*(.+)$`)
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
	value := strings.ToLower(strings.TrimSpace(input))
	value = strings.ReplaceAll(value, "/", " ")
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, ",", " ")
	value = multiSpaceRegex.ReplaceAllString(value, " ")
	value = strings.TrimSpace(value)

	if value == "" {
		return "OTHER"
	}

	compact := strings.ReplaceAll(value, " ", "")
	if containsAny(value, compact, []string{"condo", "apartment", "apt", "unit", "flat"}) {
		return "CONDO"
	}
	if containsAny(value, compact, []string{"townhouse", "row", "rowhouse", "row house", "terrace"}) {
		return "TOWNHOUSE"
	}
	if containsAny(value, compact, []string{"duplex", "semi detached", "half duplex"}) {
		return "DUPLEX"
	}
	if containsAny(value, compact, []string{"land", "lot", "vacant"}) {
		return "LAND"
	}
	if containsAny(value, compact, []string{"house", "detached", "single family", "bungalow"}) {
		return "HOUSE"
	}

	return "OTHER"
}

func NormalizePropertyTypeHint(input string) (string, bool) {
	value := strings.ToUpper(strings.TrimSpace(input))
	switch value {
	case "HOUSE", "CONDO", "TOWNHOUSE", "DUPLEX", "LAND", "OTHER":
		return value, true
	default:
		return "", false
	}
}

func containsAny(normalized string, compact string, tokens []string) bool {
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.Contains(normalized, token) {
			return true
		}
		compactToken := strings.ReplaceAll(token, " ", "")
		if compactToken != "" && strings.Contains(compact, compactToken) {
			return true
		}
	}
	return false
}

func SplitUnitFromAddress(input string) (string, string, bool) {
	match := unitPrefixRegex.FindStringSubmatch(input)
	if len(match) < 3 {
		return "", strings.TrimSpace(input), false
	}
	unit := strings.TrimSpace(match[1])
	address := strings.TrimSpace(match[2])
	if unit == "" || address == "" {
		return "", strings.TrimSpace(input), false
	}
	return unit, address, true
}
