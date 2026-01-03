package main

import (
	"context"
	"fmt"
	"log"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/app"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/upload"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	result, err := app.GenerateSnapshot(context.Background(), cfg)
	if err != nil {
		log.Fatalf("scraper error: %v", err)
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
