package outbound

import "context"

// VideoStorage is the secondary port for video file storage (S3).
type VideoStorage interface {
	Download(ctx context.Context, s3Key, destPath string) error
	Upload(ctx context.Context, filePath, s3Key string) error
}
