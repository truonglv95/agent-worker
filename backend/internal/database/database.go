package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// DB is the package-level connection pool shared across the application.
var DB *sql.DB

// Connect establishes a PostgreSQL connection pool with retry logic.
// It attempts up to 10 times with a 3-second pause between each attempt.
func Connect(databaseURL string) error {
	const maxAttempts = 10
	const retryDelay = 3 * time.Second

	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		DB, err = sql.Open("postgres", databaseURL)
		if err != nil {
			log.Printf("[database] attempt %d/%d: sql.Open error: %v", attempt, maxAttempts, err)
			time.Sleep(retryDelay)
			continue
		}

		if pingErr := DB.Ping(); pingErr != nil {
			log.Printf("[database] attempt %d/%d: ping error: %v", attempt, maxAttempts, pingErr)
			_ = DB.Close()
			DB = nil
			time.Sleep(retryDelay)
			continue
		}

		// Connection successful — configure pool settings.
		DB.SetMaxOpenConns(25)
		DB.SetMaxIdleConns(10)
		DB.SetConnMaxLifetime(5 * time.Minute)

		log.Printf("[database] connected successfully on attempt %d", attempt)
		return nil
	}

	return fmt.Errorf("database: failed to connect after %d attempts: %w", maxAttempts, err)
}
