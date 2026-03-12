package config

import (
	"fmt"
	"os"
)

type Config struct {
	SQSQueueURL  string
	S3Bucket     string
	SMTPFrom     string
	SMTPPassword string
	DB           DBConfig
}

type DBConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	Name     string
}

func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		d.Host, d.Port, d.Username, d.Password, d.Name,
	)
}

func Load() (*Config, error) {
	queueURL := os.Getenv("SQS_QUEUE_URL")
	if queueURL == "" {
		return nil, fmt.Errorf("SQS_QUEUE_URL is required")
	}
	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is required")
	}

	return &Config{
		SQSQueueURL:  queueURL,
		S3Bucket:     s3Bucket,
		SMTPFrom:     os.Getenv("SMTP_FROM"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		DB: DBConfig{
			Host:     os.Getenv("DB_HOST"),
			Port:     os.Getenv("DB_PORT"),
			Username: os.Getenv("DB_USERNAME"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     os.Getenv("DB_NAME"),
		},
	}, nil
}
