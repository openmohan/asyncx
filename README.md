# asyncx

A lightweight Go package that provides a clean interface for asynchronous task processing using Redis + asynq, with task lifecycle metadata persisted in a relational database (Postgres or MySQL).

- **Queueing**: Enqueue tasks with arbitrary JSON payloads
- **Processing**: Background workers via asynq with configurable concurrency and queues
- **Persistence**: Store and query task state (created, in_progress, completed, failed) in SQL
- **Simplicity**: Hide Redis/asynq details behind a small, focused API

## Requirements
- Go 1.20+
- Redis 5+
- A SQL database (Postgres or MySQL)

## Install

```bash
go get github.com/mohans/asyncx@latest
```

Import the package:

```go
import (
    "github.com/hibiken/asynq"
    "github.com/mohans/asyncx"
)
```

## Database schema
Apply the migration in `migrations/001_create_tasks.sql` to your database.

- The default file is MySQL-compatible (uses `DATETIME` and `TEXT`).
- A Postgres variant is included as comments in the same file (uses `TIMESTAMP` and `JSONB`).

Example (Postgres):
```bash
psql "$PG_DSN" -f migrations/001_create_tasks.sql
```

Example (MySQL):
```bash
mysql -u $USER -p -h $HOST -D $DB < migrations/001_create_tasks.sql
```

## Quick start

1) Start Redis (example via Docker):
```bash
docker run --rm -p 6379:6379 redis:7
```

2) Wire the store, client, and processor in your service:

```go
package main

import (
    "context"
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "time"

    _ "github.com/lib/pq" // or: _ "github.com/go-sql-driver/mysql"

    "github.com/hibiken/asynq"
    "github.com/mohans/asyncx"
)

type EmailPayload struct {
    UserID     int    `json:"user_id"`
    TemplateID string `json:"template_id"`
}

func main() {
    // 1) Connect DB and create store
    db, err := sql.Open("postgres", "host=localhost user=postgres password=postgres dbname=app sslmode=disable")
    if err != nil { log.Fatal(err) }
    defer db.Close()

    store := asyncx.NewSQLStore(db)

    // 2) Create enqueue client
    redis := asynq.RedisClientOpt{Addr: "127.0.0.1:6379"}
    client := asyncx.NewClient(redis, store, asyncx.ClientOptions{Queue: "default"})
    defer client.Close()

    // 3) Start processor and register handlers
    processor := asyncx.NewProcessor(redis, store, asyncx.ProcessorConfig{Concurrency: 10})
    mux := asynq.NewServeMux()

    mux.HandleFunc("email:deliver", func(ctx context.Context, t *asynq.Task) error {
        var p EmailPayload
        if err := json.Unmarshal(t.Payload(), &p); err != nil {
            return err
        }
        // Do the work
        log.Printf("sending template=%s to user=%d", p.TemplateID, p.UserID)
        time.Sleep(250 * time.Millisecond)
        return nil
    })

    // 4) Provide an HTTP endpoint to enqueue jobs
    http.HandleFunc("/enqueue", func(w http.ResponseWriter, r *http.Request) {
        info, err := client.Enqueue(r.Context(), "email:deliver", EmailPayload{UserID: 1, TemplateID: "welcome"})
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            _, _ = w.Write([]byte(err.Error()))
            return
        }
        _, _ = w.Write([]byte("enqueued task id=" + info.ID))
    })

    go func() {
        log.Println("HTTP server on :8080")
        _ = http.ListenAndServe(":8080", nil)
    }()

    // 5) Run workers (blocking)
    if err := processor.Start(mux); err != nil {
        log.Fatal(err)
    }
}
```

## Concepts and lifecycle

`asyncx` persists task metadata to `asyncx_tasks` and automatically keeps it up to date via middleware.

- **created**: inserted when `Client.Enqueue` is called
- **in_progress**: set when a worker starts processing
- **completed**: set when a handler returns `nil`
- **failed**: set when a handler returns error or panics

Columns:
- `id` (asynq task ID), `type`, `queue`, `payload_json`
- `status`, `error_msg`, `result_json`
- `created_at`, `enqueued_at`, `started_at`, `finished_at`, `updated_at`

Notes:
- By default, `result_json` is not populated automatically. If you need to persist a result payload, you can extend your handler to update your store with the result (e.g., via a custom Store implementation or by writing directly to the DB before returning). The middleware will still mark the terminal state.

## API overview

- `type Store` – persistence interface
  - `InsertCreated`, `MarkEnqueued`, `MarkStarted`, `MarkCompleted`, `MarkFailed`, `GetByID`
- `func NewSQLStore(db *sql.DB) *SQLStore` – reference SQL store (Postgres/MySQL)
- `type Client` – enqueue tasks and persist metadata
  - `func NewClient(redis asynq.RedisClientOpt, store Store, opts ClientOptions) *Client`
  - `func (c *Client) Enqueue(ctx context.Context, taskType string, payload any, options ...asynq.Option) (*asynq.TaskInfo, error)`
- `type Processor` – run workers and lifecycle tracking
  - `func NewProcessor(redis asynq.RedisClientOpt, store Store, cfg ProcessorConfig) *Processor`
  - `func (p *Processor) Start(mux *asynq.ServeMux) error`
  - `func (p *Processor) Shutdown()`

Configuration:
- `ClientOptions.Queue` – default queue for enqueued tasks
- `ProcessorConfig.Concurrency` – number of worker goroutines
- `ProcessorConfig.Queues` – weighted queues map (e.g., `{"critical": 6, "default": 3, "low": 1}`)

## Choosing a database driver

- Postgres: `github.com/lib/pq` or `github.com/jackc/pgx/v5/stdlib`
- MySQL: `github.com/go-sql-driver/mysql`

`SQLStore` uses portable SQL and falls back between `?` and `$n` placeholders to support both.

## Monitoring

- Use the asynq web UI or Inspector to view queues and task activity.
- `asynqmon` (separate project) provides dashboards for Redis/asynq.

## Testing locally

```bash
# Ensure Redis is running
redis-cli ping

# Build the package and your app
go build ./...
```

## FAQ

- **How do I store a task result payload?**
  - The middleware records status transitions. If you also want to store a result payload, you can augment your handler to update the DB record (e.g., by using your own Store implementation) before returning `nil`.

- **Can I replace the SQL store?**
  - Yes. Implement `Store` and pass it to `NewClient`/`NewProcessor`.

- **How do I configure per-task options (retry, timeout, unique, schedule)?**
  - Pass standard asynq `options` (e.g., `asynq.MaxRetry`, `asynq.Timeout`, `asynq.Unique`) to `Client.Enqueue`.

## License
MIT
