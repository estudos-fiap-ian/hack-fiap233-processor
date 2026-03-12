package postgres

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// dbQuerier abstracts the database methods used by VideoRepository, enabling testing.
type dbQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Connect opens and verifies a PostgreSQL connection.
func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return db, nil
}

// Migrate runs schema migrations required by this service.
func Migrate(db *sql.DB) error {
	_, err := db.Exec(`ALTER TABLE videos ADD COLUMN IF NOT EXISTS zip_s3_key TEXT`)
	return err
}

// VideoRepository implements outbound.VideoRepository using PostgreSQL.
type VideoRepository struct {
	db dbQuerier
}

func NewVideoRepository(db *sql.DB) *VideoRepository {
	return &VideoRepository{db: db}
}

func (r *VideoRepository) UpdateStatus(ctx context.Context, videoID int, status string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE videos SET status = $1 WHERE id = $2", status, videoID)
	return err
}

func (r *VideoRepository) UpdateStatusWithZipKey(ctx context.Context, videoID int, status, zipKey string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE videos SET status = $1, zip_s3_key = $2 WHERE id = $3",
		status, zipKey, videoID,
	)
	return err
}
