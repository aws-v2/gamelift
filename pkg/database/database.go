package database

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	pg_migrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite_migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type DB struct {
	GORM *gorm.DB
}

func ConnectPostgres(cfg Config) (*DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.Port)
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &DB{GORM: gdb}, nil
}

func ConnectSQLite(path string) (*DB, error) {
	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &DB{GORM: gdb}, nil
}

func (db *DB) Migrate(migrationPath string) error {
	sqlDB, err := db.GORM.DB()
	if err != nil {
		return err
	}

	var driver database.Driver
	var driverName string
	switch db.GORM.Dialector.Name() {
	case "postgres":
		driver, err = pg_migrate.WithInstance(sqlDB, &pg_migrate.Config{})
		driverName = "postgres"
	case "sqlite":
		driver, err = sqlite_migrate.WithInstance(sqlDB, &sqlite_migrate.Config{})
		driverName = "sqlite3"
	default:
		return fmt.Errorf("unsupported dialect for migrations: %s", db.GORM.Dialector.Name())
	}

	if err != nil {
		return fmt.Errorf("could not create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationPath,
		driverName, driver,
	)
	if err != nil {
		return fmt.Errorf("could not create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

func (db *DB) Close() error {
	sqlDB, err := db.GORM.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
