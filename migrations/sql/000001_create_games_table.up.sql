DROP TABLE IF EXISTS games;

CREATE TABLE games (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    vm_id TEXT,
    user_id TEXT NOT NULL,
    arn TEXT NOT NULL,
    folder_location TEXT,
    status TEXT NOT NULL,
    streaming_mode TEXT,
    manifest TEXT,
    storage_arn TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_games_deleted_at ON games(deleted_at);
CREATE INDEX IF NOT EXISTS idx_games_vm_id ON games(vm_id);




CREATE TABLE game_manifests (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    game_id      TEXT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    version      VARCHAR(50)  NOT NULL,
    headless_bin VARCHAR(500) NOT NULL,
    main_scene   VARCHAR(500) NOT NULL,
    player_node  VARCHAR(255) NOT NULL,
    sync_nodes   JSONB        NOT NULL DEFAULT '[]',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    UNIQUE (game_id, version)
);

CREATE INDEX idx_game_manifests_game_id ON game_manifests(game_id);
CREATE INDEX idx_game_manifests_sync_nodes ON game_manifests USING GIN(sync_nodes);