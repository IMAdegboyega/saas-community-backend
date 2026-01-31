package database

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// NewPostgresDB creates a new PostgreSQL connection using sqlx with pgx driver
// Prepared statements are disabled to work with connection poolers like PgBouncer
func NewPostgresDB(url string) (*sqlx.DB, error) {
	// Parse the connection string and configure pgx
	connConfig, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Disable prepared statement caching - critical for connection poolers (Railway/PgBouncer)
	// This prevents "prepared statement already exists" and parameter count mismatch errors
	connConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// Register the connection with stdlib
	connStr := stdlib.RegisterConnConfig(connConfig)

	// Open connection with pgx driver
	db, err := sqlx.Open("pgx", connStr)
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
