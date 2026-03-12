package application

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hack-fiap233/processor/internal/domain"
	"github.com/hack-fiap233/processor/internal/ports/outbound"
)

// VideoProcessorService is the application service that orchestrates video processing.
// It implements the inbound.VideoProcessor port.
type VideoProcessorService struct {
	storage   outbound.VideoStorage
	repo      outbound.VideoRepository
	extractor outbound.FrameExtractor
	archiver  outbound.ZipArchiver
	notifier  outbound.EmailNotifier
	s3Bucket  string
}

func NewVideoProcessorService(
	storage outbound.VideoStorage,
	repo outbound.VideoRepository,
	extractor outbound.FrameExtractor,
	archiver outbound.ZipArchiver,
	notifier outbound.EmailNotifier,
	s3Bucket string,
) *VideoProcessorService {
	return &VideoProcessorService{
		storage:   storage,
		repo:      repo,
		extractor: extractor,
		archiver:  archiver,
		notifier:  notifier,
		s3Bucket:  s3Bucket,
	}
}

const blockedEmail = "lucasxonofre@gmail.com"

// Process handles a single video processing job end-to-end.
func (s *VideoProcessorService) Process(ctx context.Context, job domain.VideoJob) error {
	log.Printf("Received job: video_id=%d s3_key=%s", job.VideoID, job.S3Key)

	if job.UserEmail == blockedEmail {
		log.Printf("Blocked email %s — refusing to process video %d", job.UserEmail, job.VideoID)
		s.repo.UpdateStatus(ctx, job.VideoID, "failed") //nolint:errcheck
		if err := s.notifier.NotifyError(ctx, job.UserEmail, job.Title); err != nil {
			log.Printf("Failed to send blocked-email error notification: %v", err)
		}
		return fmt.Errorf("video processing refused: %w", domain.ErrEmailBlocked)
	}

	if err := s.repo.UpdateStatus(ctx, job.VideoID, "processing"); err != nil {
		return fmt.Errorf("failed to update status to processing: %w", err)
	}

	start := time.Now()
	result, err := s.processVideo(ctx, job)
	processingDuration.Observe(time.Since(start).Seconds())

	if err != nil {
		s.repo.UpdateStatus(ctx, job.VideoID, "failed") //nolint:errcheck
		videosProcessed.WithLabelValues("failed").Inc()
		return fmt.Errorf("video processing failed: %w", err)
	}

	if err := s.repo.UpdateStatusWithZipKey(ctx, job.VideoID, "done", result.ZipS3Key); err != nil {
		return fmt.Errorf("failed to update status to done: %w", err)
	}

	videosProcessed.WithLabelValues("done").Inc()
	framesExtracted.Observe(float64(result.FrameCount))
	log.Printf("Video %d done — %d frames — ZIP at s3://%s/%s", job.VideoID, result.FrameCount, s.s3Bucket, result.ZipS3Key)

	if err := s.notifier.Notify(ctx, job.UserEmail, job.Title, result.FrameCount); err != nil {
		log.Printf("Failed to send email: %v", err)
	}

	return nil
}

// processVideo coordinates the download → extract → archive → upload pipeline.
func (s *VideoProcessorService) processVideo(ctx context.Context, job domain.VideoJob) (*domain.ProcessingResult, error) {
	workDir, err := os.MkdirTemp("", fmt.Sprintf("video-%d-*", job.VideoID))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	videoPath := filepath.Join(workDir, "input.mp4")
	if err := s.storage.Download(ctx, job.S3Key, videoPath); err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}

	framesDir := filepath.Join(workDir, "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create frames dir: %w", err)
	}

	frames, err := s.extractor.Extract(ctx, videoPath, framesDir)
	if err != nil {
		return nil, err
	}
	log.Printf("Extracted %d frames from video %d", len(frames), job.VideoID)

	zipPath := filepath.Join(workDir, "frames.zip")
	if err := s.archiver.Archive(frames, zipPath); err != nil {
		return nil, fmt.Errorf("failed to create ZIP: %w", err)
	}

	zipKey := fmt.Sprintf("frames/%d/frames.zip", job.VideoID)
	if err := s.storage.Upload(ctx, zipPath, zipKey); err != nil {
		return nil, fmt.Errorf("failed to upload ZIP to S3: %w", err)
	}

	return &domain.ProcessingResult{
		ZipS3Key:   zipKey,
		FrameCount: len(frames),
	}, nil
}
