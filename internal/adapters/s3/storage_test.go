package s3

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// ── Mock ──────────────────────────────────────────────────────────────────────

type mockS3API struct {
	getObjectFn func(ctx context.Context, params *awss3.GetObjectInput, optFns ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
	putObjectFn func(ctx context.Context, params *awss3.PutObjectInput, optFns ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
	lastGetKey  string
	lastPutKey  string
}

func (m *mockS3API) GetObject(ctx context.Context, params *awss3.GetObjectInput, optFns ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	if params.Key != nil {
		m.lastGetKey = *params.Key
	}
	if m.getObjectFn != nil {
		return m.getObjectFn(ctx, params, optFns...)
	}
	body := io.NopCloser(strings.NewReader("video-content"))
	return &awss3.GetObjectOutput{Body: body}, nil
}

func (m *mockS3API) PutObject(ctx context.Context, params *awss3.PutObjectInput, optFns ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	if params.Key != nil {
		m.lastPutKey = *params.Key
	}
	if m.putObjectFn != nil {
		return m.putObjectFn(ctx, params, optFns...)
	}
	return &awss3.PutObjectOutput{}, nil
}

// ── Tests: Download ───────────────────────────────────────────────────────────

func TestDownload_WritesContentToFile(t *testing.T) {
	mock := &mockS3API{}
	s := &Storage{client: mock, bucket: "my-bucket"}

	destPath := filepath.Join(t.TempDir(), "video.mp4")
	if err := s.Download(context.Background(), "uploads/video.mp4", destPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(data) != "video-content" {
		t.Errorf("expected 'video-content', got %q", string(data))
	}
}

func TestDownload_UsesCorrectS3Key(t *testing.T) {
	mock := &mockS3API{}
	s := &Storage{client: mock, bucket: "my-bucket"}

	s.Download(context.Background(), "uploads/specific-key.mp4", filepath.Join(t.TempDir(), "v.mp4")) //nolint:errcheck

	if mock.lastGetKey != "uploads/specific-key.mp4" {
		t.Errorf("expected key 'uploads/specific-key.mp4', got %q", mock.lastGetKey)
	}
}

func TestDownload_GetObjectError_ReturnsError(t *testing.T) {
	getErr := errors.New("S3 not found")
	mock := &mockS3API{
		getObjectFn: func(_ context.Context, _ *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
			return nil, getErr
		},
	}
	s := &Storage{client: mock, bucket: "my-bucket"}

	err := s.Download(context.Background(), "key", filepath.Join(t.TempDir(), "v.mp4"))
	if !errors.Is(err, getErr) {
		t.Errorf("expected S3 error, got: %v", err)
	}
}

func TestDownload_InvalidDestPath_ReturnsError(t *testing.T) {
	mock := &mockS3API{}
	s := &Storage{client: mock, bucket: "my-bucket"}

	err := s.Download(context.Background(), "key", "/nonexistent/dir/file.mp4")
	if err == nil {
		t.Fatal("expected error for invalid destination path")
	}
}

// ── Tests: Upload ─────────────────────────────────────────────────────────────

func TestUpload_SendsFileToS3(t *testing.T) {
	mock := &mockS3API{}
	s := &Storage{client: mock, bucket: "my-bucket"}

	// Create a temp file to upload.
	filePath := filepath.Join(t.TempDir(), "frames.zip")
	if err := os.WriteFile(filePath, []byte("zip-content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := s.Upload(context.Background(), filePath, "frames/1/frames.zip"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.lastPutKey != "frames/1/frames.zip" {
		t.Errorf("expected put key 'frames/1/frames.zip', got %q", mock.lastPutKey)
	}
}

func TestUpload_PutObjectError_ReturnsError(t *testing.T) {
	putErr := errors.New("S3 write denied")
	mock := &mockS3API{
		putObjectFn: func(_ context.Context, _ *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
			return nil, putErr
		},
	}
	s := &Storage{client: mock, bucket: "my-bucket"}

	filePath := filepath.Join(t.TempDir(), "f.zip")
	os.WriteFile(filePath, []byte("data"), 0644) //nolint:errcheck

	err := s.Upload(context.Background(), filePath, "key")
	if !errors.Is(err, putErr) {
		t.Errorf("expected S3 put error, got: %v", err)
	}
}

func TestUpload_NonExistentFile_ReturnsError(t *testing.T) {
	s := &Storage{client: &mockS3API{}, bucket: "my-bucket"}

	err := s.Upload(context.Background(), "/nonexistent/file.zip", "key")
	if err == nil {
		t.Fatal("expected error for non-existent source file")
	}
}

func TestNew_StoresBucket(t *testing.T) {
	s := New(nil, "test-bucket")
	if s.bucket != "test-bucket" {
		t.Errorf("expected bucket 'test-bucket', got %q", s.bucket)
	}
}
