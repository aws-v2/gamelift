package main

import (
	"fmt"
	"log"
	"backend/internal/config"
	"backend/pkg/database"
)

func main() {
	cfg := config.Load()
	db, err := database.ConnectPostgres(cfg.DB)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("Dropping schema_migrations table...")
	err = db.GORM.Exec("DROP TABLE IF EXISTS schema_migrations").Error
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Successfully dropped schema_migrations. Run the app again.")
}
