CREATE TABLE IF NOT EXISTS event_locks (
    lock_key TEXT PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_event_locks_expires_at ON event_locks (expires_at);
