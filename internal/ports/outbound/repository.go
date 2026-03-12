package outbound

import "context"

// VideoRepository is the secondary port for video persistence (PostgreSQL).
type VideoRepository interface {
	UpdateStatus(ctx context.Context, videoID int, status string) error
	UpdateStatusWithZipKey(ctx context.Context, videoID int, status, zipKey string) error
}
