package fetch

import (
	"context"
	"fmt"
	"strings"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
)

type PageFetcher interface {
	FetchAll(ctx context.Context) ([]Page, error)
}

func NewFetcher(cfg config.Config, resolver NextPageResolver) (PageFetcher, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.FetchMode))
	if mode == "" {
		mode = "http"
	}
	switch mode {
	case "http":
		return NewRealtorFetcherWithResolver(cfg, nil, resolver)
	case "browser":
		return NewBrowserFetcher(cfg, resolver)
	default:
		return nil, fmt.Errorf("unsupported fetch mode: %s", cfg.FetchMode)
	}
}
