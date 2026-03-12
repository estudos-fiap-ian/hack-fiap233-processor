package smtp

import (
	"context"
	"fmt"
	"log"
	netsmtp "net/smtp"
)

// Notifier implements outbound.EmailNotifier using Gmail SMTP.
type Notifier struct {
	from     string
	password string
	// send is injectable for testing; defaults to netsmtp.SendMail.
	send func(addr string, a netsmtp.Auth, from string, to []string, msg []byte) error
}

func New(from, password string) *Notifier {
	return &Notifier{
		from:     from,
		password: password,
		send:     netsmtp.SendMail,
	}
}

func (n *Notifier) Notify(_ context.Context, toEmail, videoTitle string, frameCount int) error {
	if toEmail == "" || n.from == "" || n.password == "" {
		log.Println("Skipping email: SMTP_FROM, SMTP_PASSWORD or user email not set")
		return nil
	}

	subject := "Seu vídeo foi processado!"
	body := fmt.Sprintf(
		"Olá!\n\nSeu vídeo \"%s\" foi processado com sucesso.\n%d frames foram extraídos e estão disponíveis para download.\n\nFIAP X",
		videoTitle, frameCount,
	)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		n.from, toEmail, subject, body)

	err := n.send(
		"smtp.gmail.com:587",
		netsmtp.PlainAuth("", n.from, n.password, "smtp.gmail.com"),
		n.from,
		[]string{toEmail},
		[]byte(msg),
	)
	if err != nil {
		log.Printf("Failed to send email to %s: %v", toEmail, err)
		return err
	}
	log.Printf("Email sent to %s", toEmail)
	return nil
}
