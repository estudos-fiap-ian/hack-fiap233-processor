package outbound

import "context"

// FrameExtractor is the secondary port for extracting frames from video files (FFmpeg).
// It returns the paths of all extracted frame files.
type FrameExtractor interface {
	Extract(ctx context.Context, videoPath, outputDir string) (framePaths []string, err error)
}
