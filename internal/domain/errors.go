package domain

import "errors"

// ErrNoFramesExtracted is returned when a video yields no extractable frames.
var ErrNoFramesExtracted = errors.New("no frames extracted from video")
