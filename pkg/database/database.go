package database

import (
	"embed"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	pg_migrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite_migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type DB struct {
	GORM   *gorm.DB
	logger *zap.SugaredLogger
}

func ConnectPostgres(cfg Config, logger *zap.SugaredLogger) (*DB, error) {
	logger.Infow("DB_CONNECT_POSTGRES", "host", cfg.Host, "port", cfg.Port, "dbname", cfg.Name, "user", cfg.User)

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.Port)

	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Errorw("DB_CONNECT_POSTGRES_FAILED", "host", cfg.Host, "port", cfg.Port, "dbname", cfg.Name, "error", err)
		return nil, err
	}

	logger.Infow("DB_CONNECT_POSTGRES_SUCCESS", "host", cfg.Host, "port", cfg.Port, "dbname", cfg.Name)
	return &DB{GORM: gdb, logger: logger}, nil
}

func ConnectSQLite(path string, logger *zap.SugaredLogger) (*DB, error) {
	logger.Infow("DB_CONNECT_SQLITE", "path", path)

	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		logger.Errorw("DB_CONNECT_SQLITE_FAILED", "path", path, "error", err)
		return nil, err
	}

	logger.Infow("DB_CONNECT_SQLITE_SUCCESS", "path", path)
	return &DB{GORM: gdb, logger: logger}, nil
}

func (db *DB) Migrate(source any, migrationPath string) error {
	dialect := db.GORM.Dialector.Name()
	db.logger.Infow("DB_MIGRATE_STARTING", "dialect", dialect, "migration_path", migrationPath)

	sqlDB, err := db.GORM.DB()
	if err != nil {
		db.logger.Errorw("DB_MIGRATE_GET_SQL_DB_FAILED", "dialect", dialect, "error", err)
		return err
	}

	var driver database.Driver
	var driverName string

	switch dialect {
	case "postgres":
		driver, err = pg_migrate.WithInstance(sqlDB, &pg_migrate.Config{})
		driverName = "postgres"
	case "sqlite":
		driver, err = sqlite_migrate.WithInstance(sqlDB, &sqlite_migrate.Config{})
		driverName = "sqlite3"
	default:
		err := fmt.Errorf("unsupported dialect for migrations: %s", dialect)
		db.logger.Errorw("DB_MIGRATE_UNSUPPORTED_DIALECT", "dialect", dialect)
		return err
	}

	if err != nil {
		db.logger.Errorw("DB_MIGRATE_DRIVER_INIT_FAILED", "dialect", dialect, "driver_name", driverName, "error", err)
		return fmt.Errorf("could not create migration driver: %w", err)
	}

	db.logger.Infow("DB_MIGRATE_DRIVER_READY", "driver_name", driverName)

	var m *migrate.Migrate
	if fs, ok := source.(embed.FS); ok {
		db.logger.Infow("DB_MIGRATE_SOURCE_IOFS", "migration_path", migrationPath)

		d, err := iofs.New(fs, migrationPath)
		if err != nil {
			db.logger.Errorw("DB_MIGRATE_IOFS_INIT_FAILED", "migration_path", migrationPath, "error", err)
			return fmt.Errorf("could not create iofs source: %w", err)
		}

		m, err = migrate.NewWithInstance("iofs", d, driverName, driver)
		if err != nil {
			db.logger.Errorw("DB_MIGRATE_INSTANCE_INIT_FAILED", "source", "iofs", "driver_name", driverName, "error", err)
			return fmt.Errorf("could not create migrate instance: %w", err)
		}
	} else {
		fileSource := "file://" + migrationPath
		db.logger.Infow("DB_MIGRATE_SOURCE_FILE", "source", fileSource)

		m, err = migrate.NewWithDatabaseInstance(fileSource, driverName, driver)
		if err != nil {
			db.logger.Errorw("DB_MIGRATE_INSTANCE_INIT_FAILED", "source", fileSource, "driver_name", driverName, "error", err)
			return fmt.Errorf("could not create migrate instance: %w", err)
		}
	}

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			db.logger.Infow("DB_MIGRATE_NO_CHANGE", "dialect", dialect, "migration_path", migrationPath)
			return nil
		}
		db.logger.Errorw("DB_MIGRATE_UP_FAILED", "dialect", dialect, "migration_path", migrationPath, "error", err)
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	db.logger.Infow("DB_MIGRATE_SUCCESS", "dialect", dialect, "migration_path", migrationPath)
	return nil
}

func (db *DB) Close() error {
	db.logger.Infow("DB_CLOSE")

	sqlDB, err := db.GORM.DB()
	if err != nil {
		db.logger.Errorw("DB_CLOSE_GET_SQL_DB_FAILED", "error", err)
		return err
	}

	if err := sqlDB.Close(); err != nil {
		db.logger.Errorw("DB_CLOSE_FAILED", "error", err)
		return err
	}

	db.logger.Infow("DB_CLOSE_SUCCESS")
	return nil
}