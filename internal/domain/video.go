package domain

// VideoJob represents a video processing task received from the message queue.
type VideoJob struct {
	VideoID   int
	S3Key     string
	Title     string
	UserEmail string
}

// ProcessingResult holds the outcome of a successful video processing job.
type ProcessingResult struct {
	ZipS3Key   string
	FrameCount int
}
