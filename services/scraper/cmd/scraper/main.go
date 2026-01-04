package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/app"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/extract"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/upload"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	result, err := app.GenerateSnapshot(context.Background(), cfg)
	if err != nil {
		blocked := errors.Is(err, extract.ErrBlockedByBotDefense)
		noRecords := errors.Is(err, app.ErrNoRecordsExtracted)
		if blocked || noRecords {
			if blocked {
				log.Printf("scraper blocked by bot defense: %v", err)
				if cfg.ScraperSaveHTMLDir == "" {
					log.Printf("set SCRAPER_SAVE_HTML_DIR to capture debug HTML")
				} else {
					log.Printf("debug HTML saved under %s (page-*.html)", cfg.ScraperSaveHTMLDir)
				}
				log.Printf("retry with BROWSER_HEADLESS=false to inspect the interstitial page")
			}
			if noRecords {
				log.Printf("scraper returned no records: %v", err)
			}
			if !cfg.ForceUploadEmpty {
				fmt.Printf("pages_fetched=%d\n", result.PagesFetched)
				fmt.Printf("records_extracted=%d\n", result.RecordsExtracted)
				fmt.Printf("output_key=%s\n", result.OutputKey)
				return
			}
			log.Printf("FORCE_UPLOAD_EMPTY=true, uploading empty snapshot")
		} else {
			log.Fatalf("scraper error: %v", err)
		}
	}

	fmt.Printf("pages_fetched=%d\n", result.PagesFetched)
	fmt.Printf("records_extracted=%d\n", result.RecordsExtracted)
	fmt.Printf("output_key=%s\n", result.OutputKey)

	if cfg.DryRun {
		return
	}

	uploader, err := upload.NewS3Uploader(context.Background(), cfg)
	if err != nil {
		log.Fatalf("uploader error: %v", err)
	}

	etag, size, err := uploader.UploadFile(context.Background(), cfg.RawBucket, result.OutputKey, result.TempPath, "application/x-ndjson")
	if err != nil {
		log.Fatalf("upload error: %v", err)
	}

	fmt.Printf("uploaded s3://%s/%s etag=%s size=%d\n", cfg.RawBucket, result.OutputKey, etag, size)
}
