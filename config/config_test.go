package config

import (
	"strings"
	"testing"
)

func TestLoad_MissingSQSQueueURL(t *testing.T) {
	t.Setenv("SQS_QUEUE_URL", "")
	t.Setenv("S3_BUCKET", "my-bucket")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing SQS_QUEUE_URL")
	}
	if !strings.Contains(err.Error(), "SQS_QUEUE_URL") {
		t.Errorf("error message should mention SQS_QUEUE_URL, got: %v", err)
	}
}

func TestLoad_MissingS3Bucket(t *testing.T) {
	t.Setenv("SQS_QUEUE_URL", "https://sqs.us-east-1.amazonaws.com/123/queue")
	t.Setenv("S3_BUCKET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing S3_BUCKET")
	}
	if !strings.Contains(err.Error(), "S3_BUCKET") {
		t.Errorf("error message should mention S3_BUCKET, got: %v", err)
	}
}

func TestLoad_AllRequiredFields(t *testing.T) {
	t.Setenv("SQS_QUEUE_URL", "https://sqs.us-east-1.amazonaws.com/123/queue")
	t.Setenv("S3_BUCKET", "my-bucket")
	t.Setenv("SMTP_FROM", "from@example.com")
	t.Setenv("SMTP_PASSWORD", "secret")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USERNAME", "user")
	t.Setenv("DB_PASSWORD", "pass")
	t.Setenv("DB_NAME", "videos")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SQSQueueURL != "https://sqs.us-east-1.amazonaws.com/123/queue" {
		t.Errorf("unexpected SQSQueueURL: %s", cfg.SQSQueueURL)
	}
	if cfg.S3Bucket != "my-bucket" {
		t.Errorf("unexpected S3Bucket: %s", cfg.S3Bucket)
	}
	if cfg.SMTPFrom != "from@example.com" {
		t.Errorf("unexpected SMTPFrom: %s", cfg.SMTPFrom)
	}
	if cfg.SMTPPassword != "secret" {
		t.Errorf("unexpected SMTPPassword: %s", cfg.SMTPPassword)
	}
	if cfg.DB.Host != "localhost" {
		t.Errorf("unexpected DB.Host: %s", cfg.DB.Host)
	}
	if cfg.DB.Port != "5432" {
		t.Errorf("unexpected DB.Port: %s", cfg.DB.Port)
	}
	if cfg.DB.Username != "user" {
		t.Errorf("unexpected DB.Username: %s", cfg.DB.Username)
	}
	if cfg.DB.Password != "pass" {
		t.Errorf("unexpected DB.Password: %s", cfg.DB.Password)
	}
	if cfg.DB.Name != "videos" {
		t.Errorf("unexpected DB.Name: %s", cfg.DB.Name)
	}
}

func TestLoad_OptionalSMTPCanBeEmpty(t *testing.T) {
	t.Setenv("SQS_QUEUE_URL", "https://sqs.us-east-1.amazonaws.com/123/queue")
	t.Setenv("S3_BUCKET", "my-bucket")
	t.Setenv("SMTP_FROM", "")
	t.Setenv("SMTP_PASSWORD", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SMTPFrom != "" {
		t.Errorf("expected empty SMTPFrom, got: %s", cfg.SMTPFrom)
	}
	if cfg.SMTPPassword != "" {
		t.Errorf("expected empty SMTPPassword, got: %s", cfg.SMTPPassword)
	}
}

func TestDBConfig_DSN_Format(t *testing.T) {
	db := DBConfig{
		Host:     "myhost",
		Port:     "5432",
		Username: "myuser",
		Password: "mypass",
		Name:     "mydb",
	}
	dsn := db.DSN()

	expected := "host=myhost port=5432 user=myuser password=mypass dbname=mydb sslmode=require"
	if dsn != expected {
		t.Errorf("expected DSN %q, got %q", expected, dsn)
	}
}

func TestDBConfig_DSN_ContainsSSLModeRequire(t *testing.T) {
	db := DBConfig{Host: "h", Port: "p", Username: "u", Password: "pw", Name: "n"}
	if !strings.Contains(db.DSN(), "sslmode=require") {
		t.Error("DSN must always include sslmode=require")
	}
}
