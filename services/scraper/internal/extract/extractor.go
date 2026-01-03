package extract

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/model"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/strategy"
)

var ErrNoExtractorMatched = errors.New("no extractor matched")

type Extractor struct {
	strategies []strategy.ExtractStrategy
}

func NewExtractor(strategies ...strategy.ExtractStrategy) *Extractor {
	if len(strategies) == 0 {
		strategies = []strategy.ExtractStrategy{
			strategy.NewJsonEmbeddedStrategy(mapListingSnapshot),
			strategy.NewDomFallbackStrategy(mapListingSnapshot),
		}
	}
	return &Extractor{strategies: strategies}
}

func (e *Extractor) Extract(ctx context.Context, html []byte, baseURL string, scrapedAt time.Time) ([]model.ListingSnapshot, string, error) {
	var lastErr error
	for _, extractor := range e.strategies {
		records, matched, err := extractor.TryExtract(ctx, html, baseURL, scrapedAt)
		if err != nil {
			if matched {
				return nil, extractor.Name(), err
			}
			lastErr = err
			continue
		}
		if matched {
			return records, extractor.Name(), nil
		}
	}
	if lastErr != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrNoExtractorMatched, lastErr)
	}
	return nil, "", ErrNoExtractorMatched
}
