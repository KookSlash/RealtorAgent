package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sylvain/realtoragent/services/parser/internal/awsclient"
	"github.com/sylvain/realtoragent/services/parser/internal/config"
	"github.com/sylvain/realtoragent/services/parser/internal/db"
	"github.com/sylvain/realtoragent/services/parser/internal/processor"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	awsCfg, err := awsclient.NewAWSConfig(ctx, cfg)
	if err != nil {
		log.Fatalf("aws config error: %v", err)
	}

	s3Client := awsclient.NewS3Client(awsCfg)
	sqsClient := awsclient.NewSQSClient(awsCfg)

	dbClient, err := db.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer dbClient.Close()

	proc, err := processor.New(ctx, cfg, s3Client, sqsClient, dbClient)
	if err != nil {
		log.Fatalf("processor error: %v", err)
	}

	if cfg.Once {
		if err := proc.ProcessOnce(ctx); err != nil {
			log.Printf("process error: %v", err)
		}
		return
	}

	if err := proc.Run(ctx); err != nil {
		log.Printf("run error: %v", err)
	}
}
