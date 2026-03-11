package main

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	_ "github.com/lib/pq"
)

var (
	db        *sql.DB
	sqsClient *sqs.Client
	s3Client  *s3.Client
	queueURL  string
	s3Bucket  string
)

type SNSEnvelope struct {
	Type    string `json:"Type"`
	Message string `json:"Message"`
}

type VideoEvent struct {
	VideoID int    `json:"video_id"`
	S3Key   string `json:"s3_key"`
	Title   string `json:"title"`
}

func main() {
	queueURL = os.Getenv("SQS_QUEUE_URL")
	if queueURL == "" {
		log.Fatal("SQS_QUEUE_URL is required")
	}
	s3Bucket = os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		log.Fatal("S3_BUCKET is required")
	}

	initDB()
	initAWS()

	// Health endpoint for Kubernetes probes
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "processor"})
		})
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	log.Println("Processor started, polling SQS...")
	poll()
}

func initAWS() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	sqsClient = sqs.NewFromConfig(cfg)
	s3Client = s3.NewFromConfig(cfg)
}

func initDB() {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USERNAME"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	if _, err = db.Exec(`ALTER TABLE videos ADD COLUMN IF NOT EXISTS zip_s3_key TEXT`); err != nil {
		log.Fatalf("Failed to run migration: %v", err)
	}

	log.Println("Connected to PostgreSQL")
}

func poll() {
	for {
		output, err := sqsClient.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			log.Printf("Error receiving messages: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, msg := range output.Messages {
			if err := processMessage(msg.Body, msg.ReceiptHandle); err != nil {
				log.Printf("Error processing message: %v", err)
			}
		}
	}
}

func processMessage(body *string, receiptHandle *string) error {
	var envelope SNSEnvelope
	if err := json.Unmarshal([]byte(*body), &envelope); err != nil {
		return fmt.Errorf("failed to parse SNS envelope: %w", err)
	}

	var event VideoEvent
	if err := json.Unmarshal([]byte(envelope.Message), &event); err != nil {
		return fmt.Errorf("failed to parse VideoEvent: %w", err)
	}

	log.Printf("Received job: video_id=%d s3_key=%s", event.VideoID, event.S3Key)

	if _, err := db.Exec("UPDATE videos SET status = 'processing' WHERE id = $1", event.VideoID); err != nil {
		return fmt.Errorf("failed to update status to processing: %w", err)
	}

	zipKey, err := processVideo(event)
	if err != nil {
		db.Exec("UPDATE videos SET status = 'failed' WHERE id = $1", event.VideoID)
		return fmt.Errorf("video processing failed: %w", err)
	}

	if _, err := db.Exec(
		"UPDATE videos SET status = 'done', zip_s3_key = $1 WHERE id = $2",
		zipKey, event.VideoID,
	); err != nil {
		return fmt.Errorf("failed to update status to done: %w", err)
	}

	log.Printf("Video %d done — ZIP at s3://%s/%s", event.VideoID, s3Bucket, zipKey)

	if _, err := sqsClient.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: receiptHandle,
	}); err != nil {
		log.Printf("Warning: failed to delete SQS message: %v", err)
	}

	return nil
}

func processVideo(event VideoEvent) (string, error) {
	workDir, err := os.MkdirTemp("", fmt.Sprintf("video-%d-*", event.VideoID))
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Download video from S3
	videoPath := filepath.Join(workDir, "input.mp4")
	if err := downloadFromS3(event.S3Key, videoPath); err != nil {
		return "", fmt.Errorf("failed to download from S3: %w", err)
	}

	// Extract 1 frame per second with ffmpeg
	framesDir := filepath.Join(workDir, "frames")
	os.MkdirAll(framesDir, 0755)

	framePattern := filepath.Join(framesDir, "frame_%04d.png")
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-vf", "fps=1", "-y", framePattern)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg error: %s: %w", string(output), err)
	}

	frames, err := filepath.Glob(filepath.Join(framesDir, "*.png"))
	if err != nil || len(frames) == 0 {
		return "", fmt.Errorf("no frames extracted")
	}
	log.Printf("Extracted %d frames from video %d", len(frames), event.VideoID)

	// Create ZIP with all frames
	zipPath := filepath.Join(workDir, "frames.zip")
	if err := createZip(frames, zipPath); err != nil {
		return "", fmt.Errorf("failed to create ZIP: %w", err)
	}

	// Upload ZIP to S3
	zipKey := fmt.Sprintf("frames/%d/frames.zip", event.VideoID)
	if err := uploadToS3(zipPath, zipKey); err != nil {
		return "", fmt.Errorf("failed to upload ZIP to S3: %w", err)
	}

	return zipKey, nil
}

func downloadFromS3(s3Key, destPath string) error {
	result, err := s3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return err
	}
	defer result.Body.Close()

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, result.Body)
	return err
}

func uploadToS3(filePath, s3Key string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(s3Key),
		Body:   f,
	})
	return err
}

func createZip(files []string, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	for _, file := range files {
		if err := addToZip(w, file); err != nil {
			return err
		}
	}
	return nil
}

func addToZip(w *zip.Writer, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.Base(filePath)
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, f)
	return err
}
