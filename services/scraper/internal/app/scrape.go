package app

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/extract"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/fetch"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/write"
)

type RunResult struct {
	OutputKey        string
	TempPath         string
	RecordsExtracted int
	PagesFetched     int
}

func GenerateSnapshot(ctx context.Context, cfg config.Config) (RunResult, error) {
	if strings.TrimSpace(cfg.SearchEntrypointURL) == "" {
		outputKey, tempPath, count, err := GenerateDummySnapshot(cfg)
		return RunResult{
			OutputKey:        outputKey,
			TempPath:         tempPath,
			RecordsExtracted: count,
			PagesFetched:     0,
		}, err
	}

	return generateEntrypointSnapshot(ctx, cfg)
}

func generateEntrypointSnapshot(ctx context.Context, cfg config.Config) (RunResult, error) {
	outputKey := config.BuildOutputKey(cfg.OutputKeyPrefix, cfg.ScrapeDate, cfg.RunID)

	file, err := os.CreateTemp("", "scraper-*.jsonl")
	if err != nil {
		return RunResult{}, err
	}
	defer file.Close()

	writer := write.NewJSONLWriter(file)
	scrapedAt := time.Now().UTC()

	fetcher := fetch.NewRealtorFetcher(cfg, nil)
	pages, err := fetcher.FetchAll(ctx)
	if err != nil {
		return RunResult{}, err
	}

	extractor := extract.NewExtractor()
	recordsCount := 0

	for _, page := range pages {
		records, strategyName, err := extractor.Extract(ctx, page.HTML, page.URL, scrapedAt)
		if err != nil {
			log.Printf("extract error url=%s: %v", page.URL, err)
			continue
		}
		log.Printf("extract url=%s strategy=%s records=%d", page.URL, strategyName, len(records))
		for _, record := range records {
			if err := writer.Write(record); err != nil {
				return RunResult{}, err
			}
			recordsCount++
		}
	}

	return RunResult{
		OutputKey:        outputKey,
		TempPath:         file.Name(),
		RecordsExtracted: recordsCount,
		PagesFetched:     len(pages),
	}, nil
}
