// Package postgres creates the application's PostgreSQL connection pool.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	// Register the database/sql PostgreSQL driver.
	_ "github.com/lib/pq"
)

const (
	maxOpenConnections = 20
	maxIdleConnections = 10
	connectionLifetime = 30 * time.Minute
)

// Open establishes and verifies a PostgreSQL connection pool.
func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	database.SetMaxOpenConns(maxOpenConnections)
	database.SetMaxIdleConns(maxIdleConnections)
	database.SetConnMaxLifetime(connectionLifetime)

	if err := database.PingContext(ctx); err != nil {
		if closeErr := database.Close(); closeErr != nil {
			return nil, fmt.Errorf("open postgres: %w", errors.Join(err, closeErr))
		}
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return database, nil
}
