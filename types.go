package asyncx

import "time"

// Status represents task processing status recorded in the database.
// Valid values: created, in_progress, completed, failed.
// Kept as string for readability in SQL and flexibility.
type Status string

const (
	StatusCreated    Status = "created"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

// TaskRecord is the persisted representation of a task lifecycle.
// It stores the essential metadata for auditing and retries.
type TaskRecord struct {
	ID          string // asynq task ID
	Type        string // asynq task type
	Queue       string // queue name
	PayloadJSON string // raw JSON payload as string
	Status      Status
	ErrorMsg    *string // last error message, if any
	ResultJSON  *string // optional task result JSON, if handler set
	CreatedAt   time.Time
	EnqueuedAt  time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time
}
