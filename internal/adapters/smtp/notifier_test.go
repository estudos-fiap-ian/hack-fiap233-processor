package smtp

import (
	"context"
	"errors"
	netsmtp "net/smtp"
	"strings"
	"testing"
)

func TestNotify_SkipsWhenEmailEmpty(t *testing.T) {
	called := false
	n := &Notifier{
		from:     "from@example.com",
		password: "pass",
		send:     func(_ string, _ netsmtp.Auth, _ string, _ []string, _ []byte) error { called = true; return nil },
	}
	err := n.Notify(context.Background(), "", "title", 10)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if called {
		t.Error("send must not be called when toEmail is empty")
	}
}

func TestNotify_SkipsWhenFromEmpty(t *testing.T) {
	called := false
	n := &Notifier{
		from:     "",
		password: "pass",
		send:     func(_ string, _ netsmtp.Auth, _ string, _ []string, _ []byte) error { called = true; return nil },
	}
	err := n.Notify(context.Background(), "to@example.com", "title", 10)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if called {
		t.Error("send must not be called when from is empty")
	}
}

func TestNotify_SkipsWhenPasswordEmpty(t *testing.T) {
	called := false
	n := &Notifier{
		from:     "from@example.com",
		password: "",
		send:     func(_ string, _ netsmtp.Auth, _ string, _ []string, _ []byte) error { called = true; return nil },
	}
	err := n.Notify(context.Background(), "to@example.com", "title", 10)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if called {
		t.Error("send must not be called when password is empty")
	}
}

func TestNotify_SendsEmailWhenAllFieldsPresent(t *testing.T) {
	called := false
	n := &Notifier{
		from:     "from@example.com",
		password: "pass",
		send:     func(_ string, _ netsmtp.Auth, _ string, _ []string, _ []byte) error { called = true; return nil },
	}
	err := n.Notify(context.Background(), "to@example.com", "My Video", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("send should be called when all credentials are present")
	}
}

func TestNotify_MessageContainsVideoTitle(t *testing.T) {
	var capturedMsg []byte
	n := &Notifier{
		from:     "from@example.com",
		password: "pass",
		send: func(_ string, _ netsmtp.Auth, _ string, _ []string, msg []byte) error {
			capturedMsg = msg
			return nil
		},
	}
	n.Notify(context.Background(), "to@example.com", "Awesome Video Title", 3) //nolint:errcheck

	if !strings.Contains(string(capturedMsg), "Awesome Video Title") {
		t.Errorf("message should contain video title, got: %s", string(capturedMsg))
	}
}

func TestNotify_MessageContainsFrameCount(t *testing.T) {
	var capturedMsg []byte
	n := &Notifier{
		from:     "from@example.com",
		password: "pass",
		send: func(_ string, _ netsmtp.Auth, _ string, _ []string, msg []byte) error {
			capturedMsg = msg
			return nil
		},
	}
	n.Notify(context.Background(), "to@example.com", "Video", 42) //nolint:errcheck

	if !strings.Contains(string(capturedMsg), "42") {
		t.Errorf("message should contain frame count 42, got: %s", string(capturedMsg))
	}
}

func TestNotify_SendsToCorrectRecipient(t *testing.T) {
	var capturedTo []string
	n := &Notifier{
		from:     "from@example.com",
		password: "pass",
		send: func(_ string, _ netsmtp.Auth, _ string, to []string, _ []byte) error {
			capturedTo = to
			return nil
		},
	}
	n.Notify(context.Background(), "recipient@example.com", "Video", 1) //nolint:errcheck

	if len(capturedTo) != 1 || capturedTo[0] != "recipient@example.com" {
		t.Errorf("expected to=[recipient@example.com], got %v", capturedTo)
	}
}

func TestNotify_SendError_IsReturned(t *testing.T) {
	sendErr := errors.New("SMTP unavailable")
	n := &Notifier{
		from:     "from@example.com",
		password: "pass",
		send:     func(_ string, _ netsmtp.Auth, _ string, _ []string, _ []byte) error { return sendErr },
	}
	err := n.Notify(context.Background(), "to@example.com", "Video", 1)
	if !errors.Is(err, sendErr) {
		t.Errorf("expected send error to be returned, got: %v", err)
	}
}

func TestNew_DefaultSendIsNotNil(t *testing.T) {
	n := New("from@example.com", "pass")
	if n.send == nil {
		t.Error("New should set a default send function")
	}
}
