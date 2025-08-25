// internal/database/connection.go
package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// DB wraps the database connection
type DB struct {
	Conn *sql.DB
}

// Connect establishes a connection to PostgreSQL database
func Connect(databaseURL string) (*DB, error) {
	conn, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{Conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.Conn != nil {
		return db.Conn.Close()
	}
	return nil
}

// Ping tests the database connection
func (db *DB) Ping() error {
	if db.Conn == nil {
		return fmt.Errorf("database connection is nil")
	}
	return db.Conn.Ping()
}

// Health returns the health status of the database connection
func (db *DB) Health() map[string]interface{} {
	stats := db.Conn.Stats()

	health := map[string]interface{}{
		"status":           "healthy",
		"open_connections": stats.OpenConnections,
		"in_use":           stats.InUse,
		"idle":             stats.Idle,
		"max_open_conns":   stats.MaxOpenConnections,
	}

	// Check if connection is working
	if err := db.Ping(); err != nil {
		health["status"] = "unhealthy"
		health["error"] = err.Error()
	}

	return health
}
