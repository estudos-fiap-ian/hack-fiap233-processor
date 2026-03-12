package main

import (
	"context"
	"log"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/hack-fiap233/processor/config"
	"github.com/hack-fiap233/processor/internal/adapters/ffmpeg"
	httpadapter "github.com/hack-fiap233/processor/internal/adapters/http"
	"github.com/hack-fiap233/processor/internal/adapters/postgres"
	s3adapter "github.com/hack-fiap233/processor/internal/adapters/s3"
	smtpadapter "github.com/hack-fiap233/processor/internal/adapters/smtp"
	sqsadapter "github.com/hack-fiap233/processor/internal/adapters/sqs"
	zipadapter "github.com/hack-fiap233/processor/internal/adapters/zip"
	"github.com/hack-fiap233/processor/internal/application"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// ── Database ──────────────────────────────────────────────────────────────
	db, err := postgres.Connect(cfg.DB.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	if err := postgres.Migrate(db); err != nil {
		log.Fatalf("Failed to run migration: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// ── AWS ───────────────────────────────────────────────────────────────────
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// ── Adapters (secondary / outbound) ───────────────────────────────────────
	storage := s3adapter.New(awss3.NewFromConfig(awsCfg), cfg.S3Bucket)
	repo := postgres.NewVideoRepository(db)
	extractor := ffmpeg.New()
	archiver := zipadapter.New()
	notifier := smtpadapter.New(cfg.SMTPFrom, cfg.SMTPPassword)

	// ── Application service ───────────────────────────────────────────────────
	processor := application.NewVideoProcessorService(storage, repo, extractor, archiver, notifier, cfg.S3Bucket)

	// ── HTTP server (metrics + health) ────────────────────────────────────────
	go httpadapter.New(":8080").Start()

	// ── SQS consumer (primary / inbound) — blocks forever ────────────────────
	sqsadapter.New(awssqs.NewFromConfig(awsCfg), cfg.SQSQueueURL, processor).Start(context.Background())
}
