// Package asyncx provides a thin, opinionated layer on top of asynq to enqueue
// and process background tasks while persisting lifecycle metadata in a
// relational database for auditing and retries.
//
// Quick start:
//  1. Create a SQL DB and apply migration in asyncx/migrations.
//  2. Wire a *sql.DB and create asyncx.NewSQLStore(db).
//  3. Create a Client with NewClient(redis, store, ...). Enqueue with Enqueue.
//  4. Create a Processor and register handlers via asynq.ServeMux.
//  5. Start the processor and it will update the store on start/finish.
package asyncx
