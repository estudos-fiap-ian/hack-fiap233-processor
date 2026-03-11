package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	_ "github.com/lib/pq"
)

var (
	db        *sql.DB
	sqsClient *sqs.Client
	queueURL  string
)

// SNSEnvelope is the wrapper that SQS receives when a message comes from SNS.
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
	log.Println("Connected to PostgreSQL")
}

func poll() {
	for {
		output, err := sqsClient.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20, // long polling
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
	// Unwrap SNS envelope
	var envelope SNSEnvelope
	if err := json.Unmarshal([]byte(*body), &envelope); err != nil {
		return fmt.Errorf("failed to parse SNS envelope: %w", err)
	}

	var event VideoEvent
	if err := json.Unmarshal([]byte(envelope.Message), &event); err != nil {
		return fmt.Errorf("failed to parse VideoEvent: %w", err)
	}

	log.Printf("Received job: video_id=%d s3_key=%s", event.VideoID, event.S3Key)

	// Update video status to processing
	_, err := db.Exec("UPDATE videos SET status = 'processing' WHERE id = $1", event.VideoID)
	if err != nil {
		return fmt.Errorf("failed to update video status: %w", err)
	}

	log.Printf("Video %d status -> processing", event.VideoID)

	// Delete message from SQS so it's not reprocessed
	_, err = sqsClient.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: receiptHandle,
	})
	if err != nil {
		log.Printf("Warning: failed to delete SQS message: %v", err)
	}

	return nil
}
