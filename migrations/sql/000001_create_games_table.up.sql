DROP TABLE IF EXISTS games;

CREATE TABLE games (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    vm_id TEXT,
    user_id TEXT NOT NULL,
    arn TEXT NOT NULL,
    folder_location TEXT,
    status TEXT NOT NULL,
    manifest TEXT,
    storage_arn TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_games_deleted_at ON games(deleted_at);
CREATE INDEX IF NOT EXISTS idx_games_vm_id ON games(vm_id);
