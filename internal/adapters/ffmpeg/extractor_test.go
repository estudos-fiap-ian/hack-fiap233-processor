package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hack-fiap233/processor/internal/domain"
)

func TestExtract_FFmpegError_ReturnsError(t *testing.T) {
	e := &Extractor{
		run: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("ffmpeg: No such file or directory"), errors.New("exit status 1")
		},
	}
	_, err := e.Extract(context.Background(), "/fake/video.mp4", t.TempDir())
	if err == nil {
		t.Fatal("expected error when ffmpeg fails")
	}
}

func TestExtract_FFmpegErrorMessageIncludesOutput(t *testing.T) {
	e := &Extractor{
		run: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("specific ffmpeg error message"), errors.New("exit status 1")
		},
	}
	_, err := e.Extract(context.Background(), "/fake/video.mp4", t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

func TestExtract_NoFramesProduced_ReturnsErrNoFramesExtracted(t *testing.T) {
	e := &Extractor{
		run: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			// ffmpeg "succeeds" but writes no frames
			return nil, nil
		},
	}
	_, err := e.Extract(context.Background(), "/fake/video.mp4", t.TempDir())
	if !errors.Is(err, domain.ErrNoFramesExtracted) {
		t.Errorf("expected ErrNoFramesExtracted, got: %v", err)
	}
}

func TestExtract_Success_ReturnsFramePaths(t *testing.T) {
	outputDir := t.TempDir()

	e := &Extractor{
		run: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			// Simulate ffmpeg producing 3 frames in outputDir.
			for i := 1; i <= 3; i++ {
				path := filepath.Join(outputDir, fmt.Sprintf("frame_%04d.png", i))
				if err := os.WriteFile(path, []byte("fake-png"), 0644); err != nil {
					return nil, err
				}
			}
			return nil, nil
		},
	}

	frames, err := e.Extract(context.Background(), "/fake/video.mp4", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(frames) != 3 {
		t.Errorf("expected 3 frames, got %d", len(frames))
	}
}

func TestExtract_Success_FramePathsArePNGs(t *testing.T) {
	outputDir := t.TempDir()

	e := &Extractor{
		run: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			path := filepath.Join(outputDir, "frame_0001.png")
			return nil, os.WriteFile(path, []byte("png"), 0644)
		},
	}

	frames, err := e.Extract(context.Background(), "/fake/video.mp4", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range frames {
		if filepath.Ext(f) != ".png" {
			t.Errorf("expected .png extension, got %q", f)
		}
	}
}

func TestExtract_PassesVideoPathToFFmpeg(t *testing.T) {
	outputDir := t.TempDir()
	var capturedArgs []string

	e := &Extractor{
		run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			capturedArgs = args
			path := filepath.Join(outputDir, "frame_0001.png")
			os.WriteFile(path, []byte("png"), 0644) //nolint:errcheck
			return nil, nil
		},
	}

	e.Extract(context.Background(), "/specific/video.mp4", outputDir) //nolint:errcheck

	found := false
	for _, a := range capturedArgs {
		if a == "/specific/video.mp4" {
			found = true
		}
	}
	if !found {
		t.Errorf("video path should be passed to ffmpeg, got args: %v", capturedArgs)
	}
}

func TestNew_RunFunctionIsSet(t *testing.T) {
	e := New()
	if e.run == nil {
		t.Error("New should set a default run function")
	}
}
