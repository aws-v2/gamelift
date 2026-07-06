package migrations

import "embed"

//go:embed sql/*.sql
var MigrationFS embed.FS
