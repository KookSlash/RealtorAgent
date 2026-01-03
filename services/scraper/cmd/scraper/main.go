package main

import (
	"fmt"
	"log"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/app"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
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
}
