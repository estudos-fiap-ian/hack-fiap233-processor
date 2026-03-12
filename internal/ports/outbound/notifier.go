package outbound

import "context"

// EmailNotifier is the secondary port for sending email notifications (SMTP).
type EmailNotifier interface {
	Notify(ctx context.Context, toEmail, videoTitle string, frameCount int) error
	NotifyError(ctx context.Context, toEmail, videoTitle string) error
}
