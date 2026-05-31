CREATE TABLE IF NOT EXISTS message_records (
    id BIGSERIAL PRIMARY KEY,
    github_id TEXT NOT NULL UNIQUE,
    feishu_message_id TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    ref TEXT,
    event_type TEXT NOT NULL,
    content TEXT,
    card_string TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    event_id BIGINT NOT NULL DEFAULT 0,
    head_sha TEXT,
    image_status TEXT DEFAULT 'done',
    avatar_url TEXT,
    workflow_started_at TIMESTAMPTZ,
    timeout_notified BOOLEAN DEFAULT FALSE,
    record_type TEXT DEFAULT 'normal',
    parent_msg_id TEXT,
    sender TEXT,
    sender_url TEXT,
    avatar_url2 TEXT
);

CREATE TABLE IF NOT EXISTS image_caches (
    url TEXT PRIMARY KEY,
    img_key TEXT NOT NULL,
    hash TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS webhook_events (
    id BIGSERIAL PRIMARY KEY,
    delivery_id TEXT UNIQUE,
    event_type TEXT NOT NULL,
    hook_id BIGINT,
    payload TEXT,
    status TEXT DEFAULT 'pending',
    retry_count INT DEFAULT 0,
    reschedule_count INT DEFAULT 0,
    head_sha TEXT,
    ref TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
