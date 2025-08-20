-- asyncx task metadata table
-- For Postgres, replace DATETIME with TIMESTAMP and JSON with JSONB if desired.

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

-- Postgres variant example:
-- CREATE TABLE IF NOT EXISTS asyncx_tasks (
--   id           VARCHAR(64) PRIMARY KEY,
--   type         VARCHAR(255) NOT NULL,
--   queue        VARCHAR(64)  NOT NULL,
--   payload_json JSONB        NOT NULL,
--   status       VARCHAR(32)  NOT NULL,
--   error_msg    TEXT         NULL,
--   result_json  JSONB        NULL,
--   created_at   TIMESTAMP    NOT NULL DEFAULT NOW(),
--   updated_at   TIMESTAMP    NULL,
--   enqueued_at  TIMESTAMP    NULL,
--   started_at   TIMESTAMP    NULL,
--   finished_at  TIMESTAMP    NULL
-- );
