ALTER TABLE games ADD COLUMN IF NOT EXISTS streaming_mode TEXT;


CREATE TABLE IF NOT EXISTS game_sessions (
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

CREATE INDEX IF NOT EXISTS idx_game_sessions_active ON game_sessions (game_id, user_id, status, expires_at);




CREATE TABLE IF NOT EXISTS sessions (
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

CREATE INDEX IF NOT EXISTS idx_sessions_active ON sessions (game_id, user_id, status, expires_at);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name='games' 
        AND column_name='id' 
        AND data_type='character varying'
    ) THEN
        ALTER TABLE games ALTER COLUMN id TYPE TEXT USING id::TEXT;
    END IF;
END $$;