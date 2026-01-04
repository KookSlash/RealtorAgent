package fetch

import (
	"encoding/json"
	"strings"
	"sync"
)

const maxJSONCaptureBytes = 5 * 1024 * 1024

type jsonCapture struct {
	mu         sync.Mutex
	candidates [][]byte
}

func (c *jsonCapture) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.candidates = nil
}

func (c *jsonCapture) Add(body []byte) {
	if len(body) == 0 {
		return
	}
	c.mu.Lock()
	c.candidates = append(c.candidates, body)
	c.mu.Unlock()
}

func (c *jsonCapture) Best() []byte {
	c.mu.Lock()
	candidates := make([][]byte, len(c.candidates))
	copy(candidates, c.candidates)
	c.mu.Unlock()

	best, _ := selectBestJSONCandidate(candidates)
	return best
}

func selectBestJSONCandidate(candidates [][]byte) ([]byte, bool) {
	var best []byte
	bestScore := jsonScore{}

	for _, candidate := range candidates {
		score, ok := scoreJSONCandidate(candidate)
		if !ok {
			continue
		}
		if score.maxResults > bestScore.maxResults || (score.maxResults == bestScore.maxResults && score.listingCount > bestScore.listingCount) {
			bestScore = score
			best = candidate
		}
	}

	if len(best) == 0 {
		return nil, false
	}
	return best, true
}

type jsonScore struct {
	maxResults   int
	listingCount int
}

func scoreJSONCandidate(payload []byte) (jsonScore, bool) {
	if len(payload) == 0 {
		return jsonScore{}, false
	}

	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return jsonScore{}, false
	}

	score := scoreJSONValue(decoded)
	if score.maxResults == 0 && score.listingCount == 0 {
		return jsonScore{}, false
	}
	return score, true
}

func scoreJSONValue(value any) jsonScore {
	score := jsonScore{}

	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			score.merge(scoreJSONValue(item))
		}
	case map[string]any:
		if looksLikeListingCandidate(typed) {
			score.listingCount++
		}
		for key, nested := range typed {
			if isListingArrayKey(key) {
				if arr, ok := nested.([]any); ok && len(arr) > score.maxResults {
					score.maxResults = len(arr)
				}
			}
			score.merge(scoreJSONValue(nested))
		}
	}

	return score
}

func (s *jsonScore) merge(other jsonScore) {
	if other.maxResults > s.maxResults {
		s.maxResults = other.maxResults
	}
	s.listingCount += other.listingCount
}

func isListingArrayKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "results", "listings", "properties":
		return true
	default:
		return false
	}
}

func looksLikeListingCandidate(data map[string]any) bool {
	score := 0
	if hasKey(data, "Property") {
		score++
	}
	if hasKey(data, "Price") || hasKey(data, "PriceUnformattedValue") {
		score++
	}
	if hasKey(data, "MlsNumber") || hasKey(data, "ListingId") || hasKey(data, "Id") {
		score++
	}
	if hasKey(data, "Latitude") || hasKey(data, "Longitude") {
		score++
	}
	if hasKey(data, "Address") || hasKey(data, "AddressText") {
		score++
	}
	if hasKey(data, "RelativeDetailsURL") || hasKey(data, "URL") || hasKey(data, "Url") {
		score++
	}
	return score >= 2
}

func hasKey(data map[string]any, key string) bool {
	_, ok := data[key]
	return ok
}

func isLikelyJSONResponse(url, contentType string) bool {
	if isJSONContentType(contentType) {
		return true
	}
	return urlLooksLikeJSON(url)
}

func isJSONContentType(contentType string) bool {
	value := strings.ToLower(contentType)
	return strings.Contains(value, "application/json") || strings.Contains(value, "+json")
}

func urlLooksLikeJSON(rawURL string) bool {
	value := strings.ToLower(strings.TrimSpace(rawURL))
	if value == "" {
		return false
	}
	if strings.Contains(value, ".json") {
		return true
	}
	if strings.Contains(value, "format=json") || strings.Contains(value, "output=json") {
		return true
	}
	if strings.Contains(value, "/api/") || strings.Contains(value, "/graphql") {
		return true
	}
	return strings.Contains(value, "json=")
}
