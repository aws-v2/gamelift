package main

import (
	"fmt"
	"log"
	"backend/internal/config"
	"backend/pkg/database"
	"github.com/golang-migrate/migrate/v4"
	pg_migrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	cfg := config.Load()
	db, err := database.ConnectPostgres(cfg.DB)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqlDB, err := db.GORM.DB()
	if err != nil {
		log.Fatal(err)
	}

	driver, err := pg_migrate.WithInstance(sqlDB, &pg_migrate.Config{})
	if err != nil {
		log.Fatal(err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations/sql",
		"postgres", driver,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Forcing database version to 0...")
	if err := m.Force(0); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Successfully forced version to 0. You can now run migrations again.")
}
