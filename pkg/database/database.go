package database

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func (db *DB) Close() error {
	// GORM doesn't explicitly Close() like sql.DB, it manages connections.
	// We can get the underlying sql.DB if needed, but for now this is a no-op.
	return nil
}
