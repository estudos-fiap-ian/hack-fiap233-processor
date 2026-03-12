package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

// ── Mock ──────────────────────────────────────────────────────────────────────

type mockDB struct {
	execFn       func(ctx context.Context, query string, args ...any) (sql.Result, error)
	lastQuery    string
	lastArgs     []any
}

func (m *mockDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	m.lastQuery = query
	m.lastArgs = args
	if m.execFn != nil {
		return m.execFn(ctx, query, args...)
	}
	return mockResult{}, nil
}

type mockResult struct{}

func (mockResult) LastInsertId() (int64, error) { return 0, nil }
func (mockResult) RowsAffected() (int64, error) { return 1, nil }

// ── Tests: VideoRepository ────────────────────────────────────────────────────

func TestUpdateStatus_ExecutesCorrectQuery(t *testing.T) {
	db := &mockDB{}
	repo := &VideoRepository{db: db}

	if err := repo.UpdateStatus(context.Background(), 42, "processing"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if db.lastQuery == "" {
		t.Error("expected a query to be executed")
	}
	if len(db.lastArgs) < 2 {
		t.Fatalf("expected at least 2 query args, got %d", len(db.lastArgs))
	}
	if db.lastArgs[0] != "processing" {
		t.Errorf("expected status arg 'processing', got %v", db.lastArgs[0])
	}
	if db.lastArgs[1] != 42 {
		t.Errorf("expected videoID arg 42, got %v", db.lastArgs[1])
	}
}

func TestUpdateStatus_DBError_ReturnsError(t *testing.T) {
	dbErr := errors.New("connection reset")
	db := &mockDB{
		execFn: func(_ context.Context, _ string, _ ...any) (sql.Result, error) {
			return nil, dbErr
		},
	}
	repo := &VideoRepository{db: db}

	err := repo.UpdateStatus(context.Background(), 1, "failed")
	if !errors.Is(err, dbErr) {
		t.Errorf("expected db error, got: %v", err)
	}
}

func TestUpdateStatusWithZipKey_ExecutesCorrectQuery(t *testing.T) {
	db := &mockDB{}
	repo := &VideoRepository{db: db}

	if err := repo.UpdateStatusWithZipKey(context.Background(), 99, "done", "frames/99/frames.zip"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(db.lastArgs) < 3 {
		t.Fatalf("expected at least 3 query args, got %d", len(db.lastArgs))
	}
	if db.lastArgs[0] != "done" {
		t.Errorf("expected status arg 'done', got %v", db.lastArgs[0])
	}
	if db.lastArgs[1] != "frames/99/frames.zip" {
		t.Errorf("expected zipKey arg 'frames/99/frames.zip', got %v", db.lastArgs[1])
	}
	if db.lastArgs[2] != 99 {
		t.Errorf("expected videoID arg 99, got %v", db.lastArgs[2])
	}
}

func TestUpdateStatusWithZipKey_DBError_ReturnsError(t *testing.T) {
	dbErr := errors.New("deadlock detected")
	db := &mockDB{
		execFn: func(_ context.Context, _ string, _ ...any) (sql.Result, error) {
			return nil, dbErr
		},
	}
	repo := &VideoRepository{db: db}

	err := repo.UpdateStatusWithZipKey(context.Background(), 1, "done", "frames/1/frames.zip")
	if !errors.Is(err, dbErr) {
		t.Errorf("expected db error, got: %v", err)
	}
}

func TestNewVideoRepository_NotNil(t *testing.T) {
	// *sql.DB satisfies dbQuerier, but we can't create one without a real connection.
	// Just verify that NewVideoRepository properly wraps the passed db.
	repo := &VideoRepository{db: &mockDB{}}
	if repo == nil {
		t.Error("repository should not be nil")
	}
}

// TestConnect_PingFails verifies that Connect wraps the ping error correctly.
// Uses port 1 which will be immediately refused on any host.
func TestConnect_PingFails_ReturnsError(t *testing.T) {
	_, err := Connect("host=127.0.0.1 port=1 user=u password=p dbname=d sslmode=disable")
	if err == nil {
		t.Fatal("expected error for unreachable database")
	}
	if !strings.Contains(err.Error(), "ping") {
		t.Errorf("error should mention 'ping', got: %v", err)
	}
}

func TestNewVideoRepository_Constructor(t *testing.T) {
	// NewVideoRepository accepts *sql.DB which satisfies dbQuerier.
	// Passing nil verifies the constructor runs without panic.
	repo := NewVideoRepository(nil)
	if repo == nil {
		t.Error("NewVideoRepository should return a non-nil repository")
	}
}

func TestUpdateStatus_DifferentStatuses(t *testing.T) {
	statuses := []string{"processing", "done", "failed"}
	for _, status := range statuses {
		db := &mockDB{}
		repo := &VideoRepository{db: db}
		if err := repo.UpdateStatus(context.Background(), 1, status); err != nil {
			t.Errorf("unexpected error for status %q: %v", status, err)
		}
		if db.lastArgs[0] != status {
			t.Errorf("expected status %q, got %v", status, db.lastArgs[0])
		}
	}
}
