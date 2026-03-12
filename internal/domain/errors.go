package domain

import "errors"

// ErrNoFramesExtracted is returned when a video yields no extractable frames.
var ErrNoFramesExtracted = errors.New("no frames extracted from video")

// ErrEmailBlocked is returned when the job's email is blocked from processing.
var ErrEmailBlocked = errors.New("email blocked from video processing")
