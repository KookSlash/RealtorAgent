package strategy

import (
	"context"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/model"
)

type ListingMapper func(raw map[string]any, baseURL string, scrapedAt time.Time) (model.ListingSnapshot, bool)

type ExtractStrategy interface {
	Name() string
	TryExtract(
		ctx context.Context,
		html []byte,
		baseURL string,
		scrapedAt time.Time,
	) (records []model.ListingSnapshot, matched bool, err error)
}
