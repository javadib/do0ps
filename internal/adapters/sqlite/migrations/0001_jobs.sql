CREATE TABLE IF NOT EXISTS jobs (
    id            TEXT PRIMARY KEY,
    tenant_id     TEXT    NOT NULL DEFAULT '',
    type          TEXT    NOT NULL,
    payload       BLOB    NOT NULL,
    status        TEXT    NOT NULL,
    attempts      INTEGER NOT NULL DEFAULT 0,
    next_retry_at INTEGER NOT NULL DEFAULT 0,
    result        BLOB,
    error         TEXT    NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jobs_status_next_retry ON jobs (status, next_retry_at);
