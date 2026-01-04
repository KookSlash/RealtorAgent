package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/extract"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/fetch"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/strategy"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/write"
)

type RunResult struct {
	OutputKey        string
	TempPath         string
	RecordsExtracted int
	PagesFetched     int
}

var ErrNoRecordsExtracted = errors.New("no records extracted")

func GenerateSnapshot(ctx context.Context, cfg config.Config) (RunResult, error) {
	if strings.EqualFold(cfg.ScraperStrategy, "zolo_ca") {
		return generateZoloSnapshot(ctx, cfg)
	}

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

	fetcher, err := fetch.NewFetcher(cfg, nil)
	if err != nil {
		return RunResult{}, err
	}
	pages, err := fetcher.FetchAll(ctx)
	if err != nil {
		return RunResult{}, err
	}

	extractor := extract.NewExtractor()
	recordsCount := 0

	for _, page := range pages {
		records, strategyName, err := extractor.ExtractWithPayload(ctx, page.HTML, page.CapturedJSON, page.URL, scrapedAt)
		if err != nil {
			if strings.EqualFold(cfg.FetchMode, "browser") && recordsCount == 0 && len(page.CapturedJSON) == 0 && extract.HasRobotsNoIndex(page.HTML) {
				return RunResult{
					OutputKey:        outputKey,
					TempPath:         file.Name(),
					RecordsExtracted: recordsCount,
					PagesFetched:     len(pages),
				}, fmt.Errorf("%w: robots noindex detected for %s", extract.ErrBlockedByBotDefense, page.URL)
			}
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

func generateZoloSnapshot(ctx context.Context, cfg config.Config) (RunResult, error) {
	outputKey := config.BuildOutputKey(cfg.OutputKeyPrefix, cfg.ScrapeDate, cfg.RunID)

	file, err := os.CreateTemp("", "scraper-zolo-*.jsonl")
	if err != nil {
		return RunResult{}, err
	}
	defer file.Close()

	writer := write.NewJSONLWriter(file)
	scrapedAt := time.Now().UTC()

	fetcher, err := fetch.NewZoloHTTPFetcher(cfg, nil)
	if err != nil {
		return RunResult{}, err
	}
	pages, err := fetcher.FetchAll(ctx)
	if err != nil {
		return RunResult{
			OutputKey:        outputKey,
			TempPath:         file.Name(),
			RecordsExtracted: 0,
			PagesFetched:     len(pages),
		}, err
	}

	extractor := extract.NewExtractor(strategy.NewZoloHTMLStrategy(cfg.ZoloBaseURL))
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

	if recordsCount == 0 {
		return RunResult{
			OutputKey:        outputKey,
			TempPath:         file.Name(),
			RecordsExtracted: recordsCount,
			PagesFetched:     len(pages),
		}, fmt.Errorf("%w: zolo_ca returned zero records", ErrNoRecordsExtracted)
	}

	return RunResult{
		OutputKey:        outputKey,
		TempPath:         file.Name(),
		RecordsExtracted: recordsCount,
		PagesFetched:     len(pages),
	}, nil
}
