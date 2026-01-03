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

	outputKey, tempPath, count, err := app.GenerateDummySnapshot(cfg)
	if err != nil {
		log.Fatalf("scraper error: %v", err)
	}

	fmt.Printf("output_key=%s\n", outputKey)
	fmt.Printf("temp_file=%s\n", tempPath)
	fmt.Printf("records=%d\n", count)

	if cfg.DryRun {
		return
	}

	uploader, err := upload.NewS3Uploader(context.Background(), cfg)
	if err != nil {
		log.Fatalf("uploader error: %v", err)
	}

	etag, size, err := uploader.UploadFile(context.Background(), cfg.RawBucket, outputKey, tempPath, "application/x-ndjson")
	if err != nil {
		log.Fatalf("upload error: %v", err)
	}

	fmt.Printf("uploaded s3://%s/%s etag=%s size=%d\n", cfg.RawBucket, outputKey, etag, size)
}
