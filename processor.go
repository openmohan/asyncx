package asyncx

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
)

// Processor manages background workers and updates Store on lifecycle events.
type Processor struct {
	server *asynq.Server
	store  Store
}

type ProcessorConfig struct {
	Concurrency int
	Queues      map[string]int
}

func NewProcessor(redisOpt asynq.RedisClientOpt, store Store, cfg ProcessorConfig) *Processor {
	con := cfg.Concurrency
	if con <= 0 {
		con = 10
	}
	qs := cfg.Queues
	if qs == nil {
		qs = map[string]int{"default": 1}
	}
	server := asynq.NewServer(redisOpt, asynq.Config{Concurrency: con, Queues: qs})
	return &Processor{server: server, store: store}
}

// Middleware to mark started/completed/failed
func (p *Processor) lifecycleMiddleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		if p.store != nil {
			if id, ok := asynq.GetTaskID(ctx); ok {
				_ = p.store.MarkStarted(ctx, id, time.Now().UTC())
			}
		}
		err := next.ProcessTask(ctx, t)
		if p.store != nil {
			if id, ok := asynq.GetTaskID(ctx); ok {
				if err != nil {
					_ = p.store.MarkFailed(ctx, id, err.Error(), time.Now().UTC())
				} else {
					_ = p.store.MarkCompleted(ctx, id, nil, time.Now().UTC())
				}
			}
		}
		return err
	})
}

// Start runs the server with provided mux/handler registrations.
// The caller should build a mux and pass it in; we wrap with middleware.
func (p *Processor) Start(mux *asynq.ServeMux) error {
	if mux == nil {
		mux = asynq.NewServeMux()
	}
	h := p.lifecycleMiddleware(mux)
	return p.server.Run(h)
}

func (p *Processor) Shutdown() { p.server.Shutdown() }
