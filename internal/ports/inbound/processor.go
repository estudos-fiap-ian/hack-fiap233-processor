package inbound

import (
	"context"

	"github.com/hack-fiap233/processor/internal/domain"
)

// VideoProcessor is the primary port that drives the application.
// External adapters (SQS, HTTP) call this interface to trigger processing.
type VideoProcessor interface {
	Process(ctx context.Context, job domain.VideoJob) error
}
