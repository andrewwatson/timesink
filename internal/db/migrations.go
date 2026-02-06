package db

import (
    "fmt"
)

type migration struct {
	version int
	sql     string
}

var migrations = []migration{
	{
		version: 1,
		sql: `
-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Clients
CREATE TABLE clients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    email TEXT,
    hourly_rate REAL NOT NULL DEFAULT 0,
    notes TEXT,
    is_archived INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Time entries with invoice locking
CREATE TABLE time_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_id INTEGER NOT NULL REFERENCES clients(id),
    description TEXT,
    start_time TEXT NOT NULL,
    end_time TEXT,
    duration_seconds INTEGER,
    hourly_rate REAL NOT NULL,
    is_billable INTEGER NOT NULL DEFAULT 1,
    is_deleted INTEGER NOT NULL DEFAULT 0,
    invoice_id INTEGER REFERENCES invoices(id),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Audit trail for entry edits
CREATE TABLE entry_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER NOT NULL REFERENCES time_entries(id),
    field_name TEXT NOT NULL,
    old_value TEXT,
    new_value TEXT,
    change_reason TEXT,
    changed_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Invoices
CREATE TABLE invoices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_number TEXT NOT NULL UNIQUE,
    client_id INTEGER NOT NULL REFERENCES clients(id),
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    subtotal REAL NOT NULL DEFAULT 0,
    tax_rate REAL NOT NULL DEFAULT 0,
    tax_amount REAL NOT NULL DEFAULT 0,
    total REAL NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'draft',
    due_date TEXT,
    paid_date TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Invoice line items
CREATE TABLE invoice_line_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_id INTEGER NOT NULL REFERENCES invoices(id),
    entry_id INTEGER NOT NULL REFERENCES time_entries(id),
    date TEXT NOT NULL,
    description TEXT NOT NULL,
    hours REAL NOT NULL,
    rate REAL NOT NULL,
    amount REAL NOT NULL
);

-- Active timer (singleton for crash recovery)
CREATE TABLE active_timer (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    client_id INTEGER NOT NULL REFERENCES clients(id),
    description TEXT,
    start_time TEXT NOT NULL,
    paused_at TEXT,
    total_paused_seconds INTEGER NOT NULL DEFAULT 0
);

-- Indexes
CREATE INDEX idx_entries_client ON time_entries(client_id);
CREATE INDEX idx_entries_start ON time_entries(start_time);
CREATE INDEX idx_entries_unbilled ON time_entries(client_id, invoice_id) WHERE invoice_id IS NULL;
CREATE INDEX idx_invoices_status ON invoices(status);
`,
	},
}

// RunMigrations applies all pending database migrations
func (db *DB) RunMigrations() error {
	// Ensure schema_version table exists
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Get current schema version
	var currentVersion int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to get current schema version: %w", err)
	}

	// Apply pending migrations in a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}

		// Execute migration SQL
		if _, err := tx.Exec(m.sql); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", m.version, err)
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", m.version); err != nil {
			return fmt.Errorf("failed to record migration %d: %w", m.version, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migrations: %w", err)
	}

	return nil
}
