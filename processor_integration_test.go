package asyncx

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	_ "modernc.org/sqlite"
)

func openTestDBIntegration(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:asyncx_it?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func startMiniRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	return s
}

func pollUntil(t *testing.T, timeout time.Duration, f func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	for {
		ok, err := f()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestProcessor_Integration_SuccessAndFailure(t *testing.T) {
	s := startMiniRedis(t)
	defer s.Close()

	db := openTestDBIntegration(t)
	defer db.Close()
	store := NewSQLStore(db)

	redis := asynq.RedisClientOpt{Addr: s.Addr()}
	processor := NewProcessor(redis, store, ProcessorConfig{Concurrency: 5, Queues: map[string]int{"default": 1}})
	mux := asynq.NewServeMux()

	type P struct {
		N int `json:"n"`
	}

	mux.HandleFunc("it:ok", func(ctx context.Context, tsk *asynq.Task) error {
		var p P
		if err := json.Unmarshal(tsk.Payload(), &p); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	mux.HandleFunc("it:fail", func(ctx context.Context, tsk *asynq.Task) error {
		return errors.New("boom")
	})

	go func() { _ = processor.Start(mux) }()
	defer processor.Shutdown()

	client := NewClient(redis, store, ClientOptions{Queue: "default"})
	defer client.Close()

	ctx := context.Background()
	okInfo, err := client.Enqueue(ctx, "it:ok", P{N: 1})
	if err != nil {
		t.Fatalf("enqueue ok: %v", err)
	}
	failInfo, err := client.Enqueue(ctx, "it:fail", P{N: 2})
	if err != nil {
		t.Fatalf("enqueue fail: %v", err)
	}

	if err := pollUntil(t, 3*time.Second, func() (bool, error) {
		rec, err := store.GetByID(ctx, okInfo.ID)
		if err != nil {
			return false, nil
		}
		return rec.Status == StatusCompleted, nil
	}); err != nil {
		t.Fatalf("ok task did not complete: %v", err)
	}
	if err := pollUntil(t, 3*time.Second, func() (bool, error) {
		rec, err := store.GetByID(ctx, failInfo.ID)
		if err != nil {
			return false, nil
		}
		return rec.Status == StatusFailed, nil
	}); err != nil {
		t.Fatalf("fail task did not fail: %v", err)
	}
}
