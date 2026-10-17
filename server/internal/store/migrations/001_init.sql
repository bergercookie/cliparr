CREATE TABLE meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE sessions (
    token_hash     TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL,
    user_name      TEXT NOT NULL,
    jellyfin_token TEXT NOT NULL,
    expires_at     INTEGER NOT NULL
);

CREATE TABLE clips (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    TEXT NOT NULL,
    item_id    TEXT NOT NULL,
    title      TEXT NOT NULL,
    start_ms   INTEGER NOT NULL,
    end_ms     INTEGER NOT NULL,
    quote      TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    -- Tags are stored as ",a,b," so a tag can be matched with LIKE '%,a,%'.
    tags       TEXT NOT NULL DEFAULT ',',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX clips_user_idx ON clips (user_id, created_at DESC);
