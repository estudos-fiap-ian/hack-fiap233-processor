package sqs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/hack-fiap233/processor/internal/domain"
	"github.com/hack-fiap233/processor/internal/ports/inbound"
)

// sqsAPI abstracts the SQS client methods used by Consumer, enabling testing.
type sqsAPI interface {
	ReceiveMessage(ctx context.Context, params *awssqs.ReceiveMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *awssqs.DeleteMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error)
}

// videoEventDTO is the JSON shape of messages published to the SQS queue (via SNS).
type videoEventDTO struct {
	VideoID   int    `json:"video_id"`
	S3Key     string `json:"s3_key"`
	Title     string `json:"title"`
	UserEmail string `json:"user_email"`
}

// snsEnvelope is the outer wrapper added by SNS when delivering to SQS.
type snsEnvelope struct {
	Type    string `json:"Type"`
	Message string `json:"Message"`
}

// Consumer polls an SQS queue and dispatches jobs to a VideoProcessor.
type Consumer struct {
	client    sqsAPI
	queueURL  string
	processor inbound.VideoProcessor
}

func New(client *awssqs.Client, queueURL string, processor inbound.VideoProcessor) *Consumer {
	return &Consumer{
		client:    client,
		queueURL:  queueURL,
		processor: processor,
	}
}

// Start begins the long-polling loop. It blocks until ctx is cancelled.
func (c *Consumer) Start(ctx context.Context) {
	log.Println("Processor started, polling SQS...")
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		output, err := c.client.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("Error receiving messages: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, msg := range output.Messages {
			if err := c.handleMessage(ctx, msg.Body, msg.ReceiptHandle); err != nil {
				log.Printf("Error processing message: %v", err)
			}
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, body, receiptHandle *string) error {
	var envelope snsEnvelope
	if err := json.Unmarshal([]byte(*body), &envelope); err != nil {
		return fmt.Errorf("failed to parse SNS envelope: %w", err)
	}

	var dto videoEventDTO
	if err := json.Unmarshal([]byte(envelope.Message), &dto); err != nil {
		return fmt.Errorf("failed to parse video event: %w", err)
	}

	job := domain.VideoJob{
		VideoID:   dto.VideoID,
		S3Key:     dto.S3Key,
		Title:     dto.Title,
		UserEmail: dto.UserEmail,
	}

	if err := c.processor.Process(ctx, job); err != nil {
		return err
	}

	if _, err := c.client.DeleteMessage(ctx, &awssqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: receiptHandle,
	}); err != nil {
		log.Printf("Warning: failed to delete SQS message: %v", err)
	}

	return nil
}
