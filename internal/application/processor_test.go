package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hack-fiap233/processor/internal/domain"
)

// ── Mocks ────────────────────────────────────────────────────────────────────

type mockStorage struct {
	downloadFn func(ctx context.Context, s3Key, destPath string) error
	uploadFn   func(ctx context.Context, filePath, s3Key string) error
}

func (m *mockStorage) Download(ctx context.Context, s3Key, destPath string) error {
	if m.downloadFn != nil {
		return m.downloadFn(ctx, s3Key, destPath)
	}
	return nil
}

func (m *mockStorage) Upload(ctx context.Context, filePath, s3Key string) error {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, filePath, s3Key)
	}
	return nil
}

type mockRepository struct {
	updateStatusFn          func(ctx context.Context, videoID int, status string) error
	updateStatusWithZipKeyFn func(ctx context.Context, videoID int, status, zipKey string) error

	statusHistory []string
	zipKey        string
}

func (m *mockRepository) UpdateStatus(ctx context.Context, videoID int, status string) error {
	m.statusHistory = append(m.statusHistory, status)
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, videoID, status)
	}
	return nil
}

func (m *mockRepository) UpdateStatusWithZipKey(ctx context.Context, videoID int, status, zipKey string) error {
	m.statusHistory = append(m.statusHistory, status)
	m.zipKey = zipKey
	if m.updateStatusWithZipKeyFn != nil {
		return m.updateStatusWithZipKeyFn(ctx, videoID, status, zipKey)
	}
	return nil
}

type mockExtractor struct {
	extractFn func(ctx context.Context, videoPath, outputDir string) ([]string, error)
}

func (m *mockExtractor) Extract(ctx context.Context, videoPath, outputDir string) ([]string, error) {
	if m.extractFn != nil {
		return m.extractFn(ctx, videoPath, outputDir)
	}
	return []string{"frame_0001.png", "frame_0002.png"}, nil
}

type mockArchiver struct {
	archiveFn func(files []string, zipPath string) error
}

func (m *mockArchiver) Archive(files []string, zipPath string) error {
	if m.archiveFn != nil {
		return m.archiveFn(files, zipPath)
	}
	return nil
}

type mockNotifier struct {
	notifyFn func(ctx context.Context, toEmail, videoTitle string, frameCount int) error
	called   bool
	lastTo   string
}

func (m *mockNotifier) Notify(ctx context.Context, toEmail, videoTitle string, frameCount int) error {
	m.called = true
	m.lastTo = toEmail
	if m.notifyFn != nil {
		return m.notifyFn(ctx, toEmail, videoTitle, frameCount)
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func newService(storage *mockStorage, repo *mockRepository, extractor *mockExtractor, archiver *mockArchiver, notifier *mockNotifier) *VideoProcessorService {
	return NewVideoProcessorService(storage, repo, extractor, archiver, notifier, "my-bucket")
}

func defaultJob() domain.VideoJob {
	return domain.VideoJob{
		VideoID:   42,
		S3Key:     "uploads/video.mp4",
		Title:     "My Video",
		UserEmail: "user@example.com",
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestProcess_Success(t *testing.T) {
	repo := &mockRepository{}
	notifier := &mockNotifier{}
	svc := newService(&mockStorage{}, repo, &mockExtractor{}, &mockArchiver{}, notifier)

	err := svc.Process(context.Background(), defaultJob())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.statusHistory) != 2 {
		t.Fatalf("expected 2 status updates, got %d: %v", len(repo.statusHistory), repo.statusHistory)
	}
	if repo.statusHistory[0] != "processing" {
		t.Errorf("first status should be 'processing', got %q", repo.statusHistory[0])
	}
	if repo.statusHistory[1] != "done" {
		t.Errorf("second status should be 'done', got %q", repo.statusHistory[1])
	}
	if !notifier.called {
		t.Error("notifier should have been called on success")
	}
	if notifier.lastTo != "user@example.com" {
		t.Errorf("notification sent to wrong email: %s", notifier.lastTo)
	}
}

func TestProcess_UpdateToProcessingFails(t *testing.T) {
	dbErr := errors.New("db connection lost")
	repo := &mockRepository{
		updateStatusFn: func(_ context.Context, _ int, _ string) error { return dbErr },
	}
	svc := newService(&mockStorage{}, repo, &mockExtractor{}, &mockArchiver{}, &mockNotifier{})

	err := svc.Process(context.Background(), defaultJob())
	if err == nil {
		t.Fatal("expected error when UpdateStatus fails")
	}
	if !errors.Is(err, dbErr) {
		t.Errorf("error should wrap db error, got: %v", err)
	}
}

func TestProcess_StorageDownloadFails(t *testing.T) {
	downloadErr := errors.New("S3 not reachable")
	storage := &mockStorage{
		downloadFn: func(_ context.Context, _, _ string) error { return downloadErr },
	}
	repo := &mockRepository{}
	svc := newService(storage, repo, &mockExtractor{}, &mockArchiver{}, &mockNotifier{})

	err := svc.Process(context.Background(), defaultJob())
	if err == nil {
		t.Fatal("expected error when download fails")
	}
	if !errors.Is(err, downloadErr) {
		t.Errorf("expected download error to be wrapped, got: %v", err)
	}
	if last := repo.statusHistory[len(repo.statusHistory)-1]; last != "failed" {
		t.Errorf("status should be 'failed' after download error, got %q", last)
	}
}

func TestProcess_ExtractorFails(t *testing.T) {
	extractErr := errors.New("ffmpeg not found")
	extractor := &mockExtractor{
		extractFn: func(_ context.Context, _, _ string) ([]string, error) {
			return nil, extractErr
		},
	}
	repo := &mockRepository{}
	svc := newService(&mockStorage{}, repo, extractor, &mockArchiver{}, &mockNotifier{})

	err := svc.Process(context.Background(), defaultJob())
	if err == nil {
		t.Fatal("expected error when extractor fails")
	}
	if !errors.Is(err, extractErr) {
		t.Errorf("expected extractor error to be wrapped, got: %v", err)
	}
	if last := repo.statusHistory[len(repo.statusHistory)-1]; last != "failed" {
		t.Errorf("status should be 'failed' after extraction error, got %q", last)
	}
}

func TestProcess_ArchiverFails(t *testing.T) {
	archiveErr := errors.New("disk full")
	archiver := &mockArchiver{
		archiveFn: func(_ []string, _ string) error { return archiveErr },
	}
	repo := &mockRepository{}
	svc := newService(&mockStorage{}, repo, &mockExtractor{}, archiver, &mockNotifier{})

	err := svc.Process(context.Background(), defaultJob())
	if err == nil {
		t.Fatal("expected error when archiver fails")
	}
	if last := repo.statusHistory[len(repo.statusHistory)-1]; last != "failed" {
		t.Errorf("status should be 'failed' after archiver error, got %q", last)
	}
}

func TestProcess_StorageUploadFails(t *testing.T) {
	uploadErr := errors.New("S3 upload refused")
	storage := &mockStorage{
		uploadFn: func(_ context.Context, _, _ string) error { return uploadErr },
	}
	repo := &mockRepository{}
	svc := newService(storage, repo, &mockExtractor{}, &mockArchiver{}, &mockNotifier{})

	err := svc.Process(context.Background(), defaultJob())
	if err == nil {
		t.Fatal("expected error when upload fails")
	}
	if last := repo.statusHistory[len(repo.statusHistory)-1]; last != "failed" {
		t.Errorf("status should be 'failed' after upload error, got %q", last)
	}
}

func TestProcess_UpdateToDoneFails(t *testing.T) {
	dbErr := errors.New("db update done failed")
	repo := &mockRepository{
		updateStatusWithZipKeyFn: func(_ context.Context, _ int, _, _ string) error { return dbErr },
	}
	svc := newService(&mockStorage{}, repo, &mockExtractor{}, &mockArchiver{}, &mockNotifier{})

	err := svc.Process(context.Background(), defaultJob())
	if err == nil {
		t.Fatal("expected error when UpdateStatusWithZipKey fails")
	}
	if !errors.Is(err, dbErr) {
		t.Errorf("expected db error to be wrapped, got: %v", err)
	}
}

func TestProcess_EmailErrorIsNonFatal(t *testing.T) {
	notifier := &mockNotifier{
		notifyFn: func(_ context.Context, _, _ string, _ int) error {
			return errors.New("SMTP down")
		},
	}
	svc := newService(&mockStorage{}, &mockRepository{}, &mockExtractor{}, &mockArchiver{}, notifier)

	// Email failure must NOT cause Process to return an error.
	err := svc.Process(context.Background(), defaultJob())
	if err != nil {
		t.Fatalf("email failure should be non-fatal, got: %v", err)
	}
}

func TestProcess_StatusSetToFailedOnProcessingError(t *testing.T) {
	repo := &mockRepository{}
	storage := &mockStorage{
		downloadFn: func(_ context.Context, _, _ string) error {
			return errors.New("download failed")
		},
	}
	svc := newService(storage, repo, &mockExtractor{}, &mockArchiver{}, &mockNotifier{})

	svc.Process(context.Background(), defaultJob()) //nolint:errcheck

	found := false
	for _, s := range repo.statusHistory {
		if s == "failed" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'failed' in status history, got: %v", repo.statusHistory)
	}
}

func TestProcess_ZipKeyContainsVideoID(t *testing.T) {
	repo := &mockRepository{}
	job := domain.VideoJob{VideoID: 99, S3Key: "uploads/v.mp4", Title: "T", UserEmail: "e@e.com"}
	svc := newService(&mockStorage{}, repo, &mockExtractor{}, &mockArchiver{}, &mockNotifier{})

	if err := svc.Process(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(repo.zipKey, "99") {
		t.Errorf("zip key should contain video ID 99, got: %q", repo.zipKey)
	}
	if !strings.HasSuffix(repo.zipKey, "frames.zip") {
		t.Errorf("zip key should end with frames.zip, got: %q", repo.zipKey)
	}
}

func TestProcess_FrameCountFromExtractor(t *testing.T) {
	extractor := &mockExtractor{
		extractFn: func(_ context.Context, _, _ string) ([]string, error) {
			return []string{"a.png", "b.png", "c.png", "d.png", "e.png"}, nil
		},
	}
	// Capture the frame count sent to the notifier.
	var capturedFrameCount int
	notifier := &mockNotifier{
		notifyFn: func(_ context.Context, _, _ string, frameCount int) error {
			capturedFrameCount = frameCount
			return nil
		},
	}
	svc := newService(&mockStorage{}, &mockRepository{}, extractor, &mockArchiver{}, notifier)

	if err := svc.Process(context.Background(), defaultJob()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFrameCount != 5 {
		t.Errorf("expected frame count 5, got %d", capturedFrameCount)
	}
}

func TestProcess_StatusTransitionOrder(t *testing.T) {
	repo := &mockRepository{}
	svc := newService(&mockStorage{}, repo, &mockExtractor{}, &mockArchiver{}, &mockNotifier{})

	if err := svc.Process(context.Background(), defaultJob()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.statusHistory) < 2 {
		t.Fatalf("expected at least 2 status transitions, got %v", repo.statusHistory)
	}
	if repo.statusHistory[0] != "processing" {
		t.Errorf("first transition must be 'processing', got %q", repo.statusHistory[0])
	}
	if repo.statusHistory[len(repo.statusHistory)-1] != "done" {
		t.Errorf("last transition must be 'done', got %q", repo.statusHistory[len(repo.statusHistory)-1])
	}
}

func TestProcess_PassesCorrectS3KeyToDownload(t *testing.T) {
	var downloadedKey string
	storage := &mockStorage{
		downloadFn: func(_ context.Context, s3Key, _ string) error {
			downloadedKey = s3Key
			return nil
		},
	}
	job := domain.VideoJob{VideoID: 1, S3Key: "uploads/specific-key.mp4", Title: "T", UserEmail: "e@e.com"}
	svc := newService(storage, &mockRepository{}, &mockExtractor{}, &mockArchiver{}, &mockNotifier{})

	svc.Process(context.Background(), job) //nolint:errcheck

	if downloadedKey != "uploads/specific-key.mp4" {
		t.Errorf("expected S3 key 'uploads/specific-key.mp4', got %q", downloadedKey)
	}
}

func TestNewVideoProcessorService_StoresS3Bucket(t *testing.T) {
	svc := NewVideoProcessorService(
		&mockStorage{}, &mockRepository{}, &mockExtractor{}, &mockArchiver{}, &mockNotifier{},
		"expected-bucket",
	)
	if svc.s3Bucket != "expected-bucket" {
		t.Errorf("expected s3Bucket 'expected-bucket', got %q", svc.s3Bucket)
	}
}

func TestProcess_MultipleJobsSucceed(t *testing.T) {
	svc := newService(&mockStorage{}, &mockRepository{}, &mockExtractor{}, &mockArchiver{}, &mockNotifier{})

	for i := 1; i <= 3; i++ {
		job := domain.VideoJob{VideoID: i, S3Key: fmt.Sprintf("uploads/video%d.mp4", i), Title: "T", UserEmail: "e@e.com"}
		if err := svc.Process(context.Background(), job); err != nil {
			t.Fatalf("job %d failed: %v", i, err)
		}
	}
}
