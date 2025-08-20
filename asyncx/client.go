package asyncx

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// Client wraps asynq.Client and a Store to persist metadata.
type Client struct {
	client *asynq.Client
	store  Store
	queue  string
}

type ClientOptions struct {
	Queue string
}

func NewClient(redisOpt asynq.RedisClientOpt, store Store, opts ClientOptions) *Client {
	q := opts.Queue
	if q == "" {
		q = "default"
	}
	return &Client{
		client: asynq.NewClient(redisOpt),
		store:  store,
		queue:  q,
	}
}

// Enqueue enqueues a task with type and arbitrary payload (will be JSON encoded).
// Returns asynq TaskInfo from enqueue and any error encountered.
func (c *Client) Enqueue(ctx context.Context, taskType string, payload any, options ...asynq.Option) (*asynq.TaskInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("nil asynq client")
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	t := asynq.NewTask(taskType, payloadBytes)
	info, err := c.client.EnqueueContext(ctx, t, append(options, asynq.Queue(c.queue))...)
	if err != nil {
		return nil, err
	}
	// Persist created record
	rec := TaskRecord{
		ID:          info.ID,
		Type:        taskType,
		Queue:       info.Queue,
		PayloadJSON: string(payloadBytes),
		Status:      StatusCreated,
		CreatedAt:   time.Now().UTC(),
		EnqueuedAt:  time.Now().UTC(),
	}
	if c.store != nil {
		_ = c.store.InsertCreated(ctx, rec)
		_ = c.store.MarkEnqueued(ctx, info.ID, info.Queue, time.Now().UTC())
	}
	return info, nil
}

func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
