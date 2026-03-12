package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/hack-fiap233/processor/internal/domain"
)

// Extractor implements outbound.FrameExtractor using the ffmpeg CLI.
type Extractor struct {
	// run is injectable for testing; defaults to exec.CommandContext.CombinedOutput.
	run func(ctx context.Context, name string, args ...string) ([]byte, error)
}

func New() *Extractor {
	return &Extractor{
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		},
	}
}

// Extract runs ffmpeg to extract one frame per second from videoPath into outputDir.
// It returns the paths of all extracted PNG files.
func (e *Extractor) Extract(ctx context.Context, videoPath, outputDir string) ([]string, error) {
	framePattern := filepath.Join(outputDir, "frame_%04d.png")
	output, err := e.run(ctx, "ffmpeg", "-i", videoPath, "-vf", "fps=1", "-y", framePattern)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg error: %s: %w", string(output), err)
	}

	frames, err := filepath.Glob(filepath.Join(outputDir, "*.png"))
	if err != nil || len(frames) == 0 {
		return nil, domain.ErrNoFramesExtracted
	}

	return frames, nil
}
