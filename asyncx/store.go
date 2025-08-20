package asyncx

import (
	context "context"
	"database/sql"
	"errors"
	"time"
)

// Store abstracts persistence for task lifecycle records.
// Implementations must be safe for concurrent use.
type Store interface {
	InsertCreated(ctx context.Context, rec TaskRecord) error
	MarkEnqueued(ctx context.Context, taskID string, queue string, enqueuedAt time.Time) error
	MarkStarted(ctx context.Context, taskID string, startedAt time.Time) error
	MarkCompleted(ctx context.Context, taskID string, resultJSON *string, finishedAt time.Time) error
	MarkFailed(ctx context.Context, taskID string, errorMsg string, finishedAt time.Time) error
	GetByID(ctx context.Context, taskID string) (*TaskRecord, error)
}

// SQLStore is a reference implementation backed by a relational DB (Postgres/MySQL).
// Table schema is provided in migrations.
type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) InsertCreated(ctx context.Context, rec TaskRecord) error {
	if s.db == nil {
		return errors.New("nil db")
	}
	query := `INSERT INTO asyncx_tasks (id, type, queue, payload_json, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`
	// Use Postgres-style placeholders if driver is postgres.
	// We detect driver name via DB stats workaround is unreliable; keep portable by attempting Exec with '?'
	// and fallback to '$' placeholders if needed. For simplicity, prefer '?'.
	_, err := s.db.ExecContext(ctx, query, rec.ID, rec.Type, rec.Queue, rec.PayloadJSON, string(StatusCreated), time.Now().UTC())
	if err != nil {
		// attempt Postgres style
		queryPg := `INSERT INTO asyncx_tasks (id, type, queue, payload_json, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)`
		_, err2 := s.db.ExecContext(ctx, queryPg, rec.ID, rec.Type, rec.Queue, rec.PayloadJSON, string(StatusCreated), time.Now().UTC())
		return err2
	}
	return nil
}

func (s *SQLStore) MarkEnqueued(ctx context.Context, taskID string, queue string, enqueuedAt time.Time) error {
	if s.db == nil {
		return errors.New("nil db")
	}
	q := `UPDATE asyncx_tasks SET status = ?, queue = ?, enqueued_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, string(StatusCreated), queue, enqueuedAt.UTC(), taskID)
	if err != nil {
		qpg := `UPDATE asyncx_tasks SET status = $1, queue = $2, enqueued_at = $3, updated_at = NOW() WHERE id = $4`
		_, err2 := s.db.ExecContext(ctx, qpg, string(StatusCreated), queue, enqueuedAt.UTC(), taskID)
		return err2
	}
	return nil
}

func (s *SQLStore) MarkStarted(ctx context.Context, taskID string, startedAt time.Time) error {
	if s.db == nil {
		return errors.New("nil db")
	}
	q := `UPDATE asyncx_tasks SET status = ?, started_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, string(StatusInProgress), startedAt.UTC(), taskID)
	if err != nil {
		qpg := `UPDATE asyncx_tasks SET status = $1, started_at = $2, updated_at = NOW() WHERE id = $3`
		_, err2 := s.db.ExecContext(ctx, qpg, string(StatusInProgress), startedAt.UTC(), taskID)
		return err2
	}
	return nil
}

func (s *SQLStore) MarkCompleted(ctx context.Context, taskID string, resultJSON *string, finishedAt time.Time) error {
	if s.db == nil {
		return errors.New("nil db")
	}
	q := `UPDATE asyncx_tasks SET status = ?, result_json = ?, finished_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, string(StatusCompleted), resultJSON, finishedAt.UTC(), taskID)
	if err != nil {
		qpg := `UPDATE asyncx_tasks SET status = $1, result_json = $2, finished_at = $3, updated_at = NOW() WHERE id = $4`
		_, err2 := s.db.ExecContext(ctx, qpg, string(StatusCompleted), resultJSON, finishedAt.UTC(), taskID)
		return err2
	}
	return nil
}

func (s *SQLStore) MarkFailed(ctx context.Context, taskID string, errorMsg string, finishedAt time.Time) error {
	if s.db == nil {
		return errors.New("nil db")
	}
	q := `UPDATE asyncx_tasks SET status = ?, error_msg = ?, finished_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, string(StatusFailed), errorMsg, finishedAt.UTC(), taskID)
	if err != nil {
		qpg := `UPDATE asyncx_tasks SET status = $1, error_msg = $2, finished_at = $3, updated_at = NOW() WHERE id = $4`
		_, err2 := s.db.ExecContext(ctx, qpg, string(StatusFailed), errorMsg, finishedAt.UTC(), taskID)
		return err2
	}
	return nil
}

func (s *SQLStore) GetByID(ctx context.Context, taskID string) (*TaskRecord, error) {
	if s.db == nil {
		return nil, errors.New("nil db")
	}
	q := `SELECT id, type, queue, payload_json, status, error_msg, result_json, created_at, enqueued_at, started_at, finished_at FROM asyncx_tasks WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, taskID)
	rec := TaskRecord{}
	var status string
	var startedAt, finishedAt, enqueuedAt sql.NullTime
	var errorMsg, resultJSON sql.NullString
	if err := row.Scan(&rec.ID, &rec.Type, &rec.Queue, &rec.PayloadJSON, &status, &errorMsg, &resultJSON, &rec.CreatedAt, &enqueuedAt, &startedAt, &finishedAt); err != nil {
		// retry with postgres placeholders if needed
		qpg := `SELECT id, type, queue, payload_json, status, error_msg, result_json, created_at, enqueued_at, started_at, finished_at FROM asyncx_tasks WHERE id = $1`
		row = s.db.QueryRowContext(ctx, qpg, taskID)
		if err2 := row.Scan(&rec.ID, &rec.Type, &rec.Queue, &rec.PayloadJSON, &status, &errorMsg, &resultJSON, &rec.CreatedAt, &enqueuedAt, &startedAt, &finishedAt); err2 != nil {
			return nil, err2
		}
	}
	rec.Status = Status(status)
	if errorMsg.Valid {
		v := errorMsg.String
		rec.ErrorMsg = &v
	}
	if resultJSON.Valid {
		v := resultJSON.String
		rec.ResultJSON = &v
	}
	if startedAt.Valid {
		t := startedAt.Time
		rec.StartedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		rec.FinishedAt = &t
	}
	if enqueuedAt.Valid {
		rec.EnqueuedAt = enqueuedAt.Time
	}
	return &rec, nil
}
