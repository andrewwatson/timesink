package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mutecomm/go-sqlcipher/v4"
)

type DB struct {
	*sql.DB
}

// Open opens an encrypted SQLite database with the given password.
// dbPath is the full path to the database file.
func Open(dbPath, password string) (*DB, error) {
	// Create parent directories if they don't exist
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Build connection string with encryption key
	connStr := fmt.Sprintf("%s?_key=%s", dbPath, password)

	// Open the database
	sqlDB, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Enable WAL mode for better concurrent performance
	if _, err := sqlDB.Exec("PRAGMA journal_mode = WAL"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Ping to verify connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: sqlDB}, nil
}

// OpenWithDefaults opens the database at the default location
// ~/.config/timesink/timesink.db
func OpenWithDefaults(password string) (*DB, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	dbPath := filepath.Join(homeDir, ".config", "timesink", "timesink.db")
	return Open(dbPath, password)
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
