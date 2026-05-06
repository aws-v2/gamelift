ALTER TABLE games ADD COLUMN IF NOT EXISTS streaming_mode TEXT;


CREATE TABLE game_sessions (
    id           TEXT        PRIMARY KEY,
    game_id      TEXT        NOT NULL,
    user_id      TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'provisioning',
    token        TEXT        NOT NULL,
    node_id      TEXT        NOT NULL DEFAULT '',
    agent_ws_url TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_game_sessions_active ON game_sessions (game_id, user_id, status, expires_at);




CREATE TABLE sessions (
    id           TEXT        PRIMARY KEY,
    game_id      TEXT        NOT NULL,
    user_id      TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'provisioning',
    token        TEXT        NOT NULL,
    node_id      TEXT        NOT NULL DEFAULT '',
    agent_ws_url TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_sessions_active ON sessions (game_id, user_id, status, expires_at);