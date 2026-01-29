package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// NewPostgresDB creates a new PostgreSQL connection using sqlx
func NewPostgresDB(url string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	// Short connection lifetime to avoid prepared statement caching issues
	// when queries change between deployments
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(2)                    // Reduced from 5
	db.SetConnMaxLifetime(1 * time.Minute)   // Reduced from 5 minutes
	db.SetConnMaxIdleTime(30 * time.Second)  // Reduced from 1 minute

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// NewPostgresDBStandard creates a standard sql.DB connection (for compatibility)
func NewPostgresDBStandard(url string) (*sql.DB, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
