package sqs

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/hack-fiap233/processor/internal/domain"
)

// ── Mocks ─────────────────────────────────────────────────────────────────────

type mockSQSAPI struct {
	receiveMessageFn func(ctx context.Context, params *awssqs.ReceiveMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error)
	deleteMessageFn  func(ctx context.Context, params *awssqs.DeleteMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error)
	deleteCalled     bool
}

func (m *mockSQSAPI) ReceiveMessage(ctx context.Context, params *awssqs.ReceiveMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
	if m.receiveMessageFn != nil {
		return m.receiveMessageFn(ctx, params, optFns...)
	}
	return &awssqs.ReceiveMessageOutput{}, nil
}

func (m *mockSQSAPI) DeleteMessage(ctx context.Context, params *awssqs.DeleteMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error) {
	m.deleteCalled = true
	if m.deleteMessageFn != nil {
		return m.deleteMessageFn(ctx, params, optFns...)
	}
	return &awssqs.DeleteMessageOutput{}, nil
}

type mockProcessor struct {
	processFn     func(ctx context.Context, job domain.VideoJob) error
	processedJobs []domain.VideoJob
}

func (m *mockProcessor) Process(ctx context.Context, job domain.VideoJob) error {
	m.processedJobs = append(m.processedJobs, job)
	if m.processFn != nil {
		return m.processFn(ctx, job)
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func ptr(s string) *string { return &s }

func buildMessage(videoID int, s3Key, title, userEmail string) string {
	inner := fmt.Sprintf(`{"video_id":%d,"s3_key":%q,"title":%q,"user_email":%q}`, videoID, s3Key, title, userEmail)
	return fmt.Sprintf(`{"Type":"Notification","Message":%q}`, inner)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestNew_Constructor(t *testing.T) {
	proc := &mockProcessor{}
	c := New(nil, "my-queue-url", proc)
	if c == nil {
		t.Fatal("New should return a non-nil Consumer")
	}
	if c.queueURL != "my-queue-url" {
		t.Errorf("expected queueURL 'my-queue-url', got %q", c.queueURL)
	}
}

func TestHandleMessage_ValidMessage_CallsProcessor(t *testing.T) {
	proc := &mockProcessor{}
	c := &Consumer{client: &mockSQSAPI{}, queueURL: "url", processor: proc}

	body := buildMessage(42, "uploads/video.mp4", "My Video", "user@example.com")
	err := c.handleMessage(context.Background(), ptr(body), ptr("receipt-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(proc.processedJobs) != 1 {
		t.Fatalf("expected 1 processed job, got %d", len(proc.processedJobs))
	}
}

func TestHandleMessage_MapsAllFields(t *testing.T) {
	proc := &mockProcessor{}
	c := &Consumer{client: &mockSQSAPI{}, queueURL: "url", processor: proc}

	body := buildMessage(77, "uploads/specific.mp4", "Specific Title", "specific@example.com")
	if err := c.handleMessage(context.Background(), ptr(body), ptr("r")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	job := proc.processedJobs[0]
	if job.VideoID != 77 {
		t.Errorf("expected VideoID 77, got %d", job.VideoID)
	}
	if job.S3Key != "uploads/specific.mp4" {
		t.Errorf("expected S3Key 'uploads/specific.mp4', got %q", job.S3Key)
	}
	if job.Title != "Specific Title" {
		t.Errorf("expected Title 'Specific Title', got %q", job.Title)
	}
	if job.UserEmail != "specific@example.com" {
		t.Errorf("expected UserEmail 'specific@example.com', got %q", job.UserEmail)
	}
}

func TestHandleMessage_InvalidEnvelopeJSON_ReturnsError(t *testing.T) {
	c := &Consumer{client: &mockSQSAPI{}, queueURL: "url", processor: &mockProcessor{}}

	err := c.handleMessage(context.Background(), ptr("not-json"), ptr("r"))
	if err == nil {
		t.Fatal("expected error for invalid envelope JSON")
	}
}

func TestHandleMessage_InvalidInnerJSON_ReturnsError(t *testing.T) {
	c := &Consumer{client: &mockSQSAPI{}, queueURL: "url", processor: &mockProcessor{}}

	body := `{"Type":"Notification","Message":"not-valid-json"}`
	err := c.handleMessage(context.Background(), ptr(body), ptr("r"))
	if err == nil {
		t.Fatal("expected error for invalid inner JSON")
	}
}

func TestHandleMessage_ProcessorError_DoesNotDeleteMessage(t *testing.T) {
	sqsMock := &mockSQSAPI{}
	proc := &mockProcessor{
		processFn: func(_ context.Context, _ domain.VideoJob) error {
			return errors.New("processing failed")
		},
	}
	c := &Consumer{client: sqsMock, queueURL: "url", processor: proc}

	body := buildMessage(1, "k", "t", "e@e.com")
	err := c.handleMessage(context.Background(), ptr(body), ptr("r"))
	if err == nil {
		t.Fatal("expected error from processor")
	}
	if sqsMock.deleteCalled {
		t.Error("message should NOT be deleted when processor returns an error")
	}
}

func TestHandleMessage_DeletesMessageOnSuccess(t *testing.T) {
	sqsMock := &mockSQSAPI{}
	c := &Consumer{client: sqsMock, queueURL: "url", processor: &mockProcessor{}}

	body := buildMessage(1, "k", "t", "e@e.com")
	if err := c.handleMessage(context.Background(), ptr(body), ptr("r")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sqsMock.deleteCalled {
		t.Error("message should be deleted after successful processing")
	}
}

func TestHandleMessage_DeleteError_IsNonFatal(t *testing.T) {
	sqsMock := &mockSQSAPI{
		deleteMessageFn: func(_ context.Context, _ *awssqs.DeleteMessageInput, _ ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error) {
			return nil, errors.New("delete failed")
		},
	}
	c := &Consumer{client: sqsMock, queueURL: "url", processor: &mockProcessor{}}

	body := buildMessage(1, "k", "t", "e@e.com")
	// Delete failure must NOT bubble up as an error.
	err := c.handleMessage(context.Background(), ptr(body), ptr("r"))
	if err != nil {
		t.Fatalf("delete error should be non-fatal, got: %v", err)
	}
}

func TestStart_ExitsWhenContextCancelledDuringReceive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sqsMock := &mockSQSAPI{
		receiveMessageFn: func(_ context.Context, _ *awssqs.ReceiveMessageInput, _ ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
			// Cancel the context and return a context error to trigger the
			// `if ctx.Err() != nil { return }` branch inside Start.
			cancel()
			return nil, context.Canceled
		},
	}
	c := &Consumer{client: sqsMock, queueURL: "url", processor: &mockProcessor{}}

	done := make(chan struct{})
	go func() {
		c.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not exit after context cancellation during ReceiveMessage")
	}
}

func TestStart_LogsMessageHandlingError(t *testing.T) {
	// When handleMessage returns an error, Start must log it and continue (not crash).
	proc := &mockProcessor{
		processFn: func(_ context.Context, _ domain.VideoJob) error {
			return errors.New("processing failed")
		},
	}
	callCount := 0
	ctx, cancel := context.WithCancel(context.Background())
	receipt := "r"
	sqsMock := &mockSQSAPI{
		receiveMessageFn: func(_ context.Context, _ *awssqs.ReceiveMessageInput, _ ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
			callCount++
			if callCount == 1 {
				body := buildMessage(1, "key", "title", "e@e.com")
				return &awssqs.ReceiveMessageOutput{
					Messages: []types.Message{{Body: &body, ReceiptHandle: &receipt}},
				}, nil
			}
			cancel()
			return &awssqs.ReceiveMessageOutput{}, nil
		},
	}
	c := &Consumer{client: sqsMock, queueURL: "url", processor: proc}

	done := make(chan struct{})
	go func() {
		c.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not exit")
	}
}

func TestStart_StopsOnContextCancellation(t *testing.T) {
	sqsMock := &mockSQSAPI{
		receiveMessageFn: func(ctx context.Context, _ *awssqs.ReceiveMessageInput, _ ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
			// Simulate a fast return when context is cancelled
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return &awssqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil
		},
	}
	c := &Consumer{client: sqsMock, queueURL: "url", processor: &mockProcessor{}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Start

	done := make(chan struct{})
	go func() {
		c.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// OK: Start exited after context cancellation
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not exit after context cancellation")
	}
}

func TestStart_ProcessesMessagesFromQueue(t *testing.T) {
	callCount := 0
	proc := &mockProcessor{}
	receipt := "receipt-handle-1"

	ctx, cancel := context.WithCancel(context.Background())

	sqsMock := &mockSQSAPI{
		receiveMessageFn: func(_ context.Context, _ *awssqs.ReceiveMessageInput, _ ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
			callCount++
			if callCount == 1 {
				body := buildMessage(10, "key", "title", "e@e.com")
				return &awssqs.ReceiveMessageOutput{
					Messages: []types.Message{{Body: &body, ReceiptHandle: &receipt}},
				}, nil
			}
			// Cancel the context to stop the loop on the second poll.
			cancel()
			return &awssqs.ReceiveMessageOutput{}, nil
		},
	}
	c := &Consumer{client: sqsMock, queueURL: "url", processor: proc}

	done := make(chan struct{})
	go func() {
		c.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not exit")
	}

	if len(proc.processedJobs) != 1 {
		t.Errorf("expected 1 processed job, got %d", len(proc.processedJobs))
	}
}
