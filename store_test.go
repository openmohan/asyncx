package asyncx

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS asyncx_tasks (
    id           VARCHAR(64) PRIMARY KEY,
    type         VARCHAR(255) NOT NULL,
    queue        VARCHAR(64)  NOT NULL,
    payload_json TEXT         NOT NULL,
    status       VARCHAR(32)  NOT NULL,
    error_msg    TEXT         NULL,
    result_json  TEXT         NULL,
    created_at   DATETIME     NOT NULL,
    updated_at   DATETIME     NULL,
    enqueued_at  DATETIME     NULL,
    started_at   DATETIME     NULL,
    finished_at  DATETIME     NULL
);
`

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:asyncx_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func TestSQLStore_Lifecycle_Success(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	store := NewSQLStore(db)
	ctx := context.Background()

	payload := map[string]any{"user_id": 123}
	payloadBytes, _ := json.Marshal(payload)

	rec := TaskRecord{
		ID:          "task-1",
		Type:        "email:deliver",
		Queue:       "default",
		PayloadJSON: string(payloadBytes),
		Status:      StatusCreated,
		CreatedAt:   time.Now().UTC(),
	}
	if err := store.InsertCreated(ctx, rec); err != nil {
		t.Fatalf("InsertCreated: %v", err)
	}
	if err := store.MarkEnqueued(ctx, rec.ID, rec.Queue, time.Now().UTC()); err != nil {
		t.Fatalf("MarkEnqueued: %v", err)
	}
	if err := store.MarkStarted(ctx, rec.ID, time.Now().UTC()); err != nil {
		t.Fatalf("MarkStarted: %v", err)
	}
	result := `{"ok":true}`
	if err := store.MarkCompleted(ctx, rec.ID, &result, time.Now().UTC()); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}

	got, err := store.GetByID(ctx, rec.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != rec.ID {
		t.Fatalf("unexpected record: %#v", got)
	}
	if got.Status != StatusCompleted {
		t.Fatalf("want status=%s got=%s", StatusCompleted, got.Status)
	}
	if got.ResultJSON == nil || *got.ResultJSON != result {
		t.Fatalf("unexpected result json: %#v", got.ResultJSON)
	}
	if got.StartedAt == nil || got.FinishedAt == nil {
		t.Fatalf("expected timestamps to be set: started=%v finished=%v", got.StartedAt, got.FinishedAt)
	}
}

func TestSQLStore_MarkFailed(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	store := NewSQLStore(db)
	ctx := context.Background()

	rec := TaskRecord{ID: "task-2", Type: "email:deliver", Queue: "default", PayloadJSON: `{}`, Status: StatusCreated, CreatedAt: time.Now().UTC()}
	if err := store.InsertCreated(ctx, rec); err != nil {
		t.Fatalf("InsertCreated: %v", err)
	}
	errMsg := "boom"
	if err := store.MarkFailed(ctx, rec.ID, errMsg, time.Now().UTC()); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	got, err := store.GetByID(ctx, rec.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("want status=%s got=%s", StatusFailed, got.Status)
	}
	if got.ErrorMsg == nil || *got.ErrorMsg != errMsg {
		t.Fatalf("unexpected error msg: %#v", got.ErrorMsg)
	}
}
