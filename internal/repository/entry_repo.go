package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/andy/timesink/internal/db"
	"github.com/andy/timesink/internal/domain"
)

// EntryRepo is a SQLite implementation of TimeEntryRepository
type EntryRepo struct {
	db *db.DB
}

// NewEntryRepo creates a new EntryRepo
func NewEntryRepo(database *db.DB) *EntryRepo {
	return &EntryRepo{db: database}
}

// Create inserts a new time entry into the database
func (r *EntryRepo) Create(ctx context.Context, entry *domain.TimeEntry) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid time entry: %w", err)
	}

	query := `
		INSERT INTO time_entries (
			client_id, description, start_time, end_time, duration_seconds,
			hourly_rate, is_billable, is_deleted, invoice_id, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var endTime, durationSeconds interface{}
	if entry.EndTime != nil {
		endTime = entry.EndTime.Format(timeLayout)
	}
	if entry.DurationSeconds != nil {
		durationSeconds = *entry.DurationSeconds
	}

	result, err := r.db.ExecContext(ctx, query,
		entry.ClientID,
		entry.Description,
		entry.StartTime.Format(timeLayout),
		endTime,
		durationSeconds,
		entry.HourlyRate,
		entry.IsBillable,
		entry.IsDeleted,
		entry.InvoiceID,
		entry.CreatedAt.Format(timeLayout),
		entry.UpdatedAt.Format(timeLayout),
	)
	if err != nil {
		return fmt.Errorf("failed to create time entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get time entry ID: %w", err)
	}

	entry.ID = id
	return nil
}

// GetByID retrieves a time entry by ID
func (r *EntryRepo) GetByID(ctx context.Context, id int64) (*domain.TimeEntry, error) {
	query := `
		SELECT id, client_id, description, start_time, end_time, duration_seconds,
		       hourly_rate, is_billable, is_deleted, invoice_id, created_at, updated_at
		FROM time_entries
		WHERE id = ?
	`

	entry := &domain.TimeEntry{}
	var startTime, createdAt, updatedAt sql.NullString
	var endTime, durationSeconds, invoiceID sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&entry.ID,
		&entry.ClientID,
		&entry.Description,
		&startTime,
		&endTime,
		&durationSeconds,
		&entry.HourlyRate,
		&entry.IsBillable,
		&entry.IsDeleted,
		&invoiceID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("time entry not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get time entry: %w", err)
	}

	if err := scanTimeEntry(entry, startTime, endTime, durationSeconds, invoiceID, createdAt, updatedAt); err != nil {
		return nil, err
	}

	return entry, nil
}

// Update updates an existing time entry and creates an audit record
func (r *EntryRepo) Update(ctx context.Context, entry *domain.TimeEntry, reason string) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid time entry: %w", err)
	}

	// Check if entry is locked
	locked, err := r.IsLocked(ctx, entry.ID)
	if err != nil {
		return err
	}
	if locked {
		return fmt.Errorf("cannot update time entry: locked by invoice")
	}

	// Get current entry for audit trail
	oldEntry, err := r.GetByID(ctx, entry.ID)
	if err != nil {
		return err
	}

	// Begin transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update the entry
	query := `
		UPDATE time_entries
		SET client_id = ?, description = ?, start_time = ?, end_time = ?, duration_seconds = ?,
		    hourly_rate = ?, is_billable = ?, updated_at = ?
		WHERE id = ? AND is_deleted = 0
	`

	var endTime, durationSeconds interface{}
	if entry.EndTime != nil {
		endTime = entry.EndTime.Format(timeLayout)
	}
	if entry.DurationSeconds != nil {
		durationSeconds = *entry.DurationSeconds
	}

	entry.UpdatedAt = time.Now()

	result, err := tx.ExecContext(ctx, query,
		entry.ClientID,
		entry.Description,
		entry.StartTime.Format(timeLayout),
		endTime,
		durationSeconds,
		entry.HourlyRate,
		entry.IsBillable,
		entry.UpdatedAt.Format(timeLayout),
		entry.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update time entry: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("time entry not found or already deleted")
	}

	// Create audit records for changed fields
	if err := r.createAuditRecords(ctx, tx, oldEntry, entry, reason); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SoftDelete marks a time entry as deleted
func (r *EntryRepo) SoftDelete(ctx context.Context, id int64, reason string) error {
	// Check if entry is locked
	locked, err := r.IsLocked(ctx, id)
	if err != nil {
		return err
	}
	if locked {
		return fmt.Errorf("cannot delete time entry: locked by invoice")
	}

	// Begin transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		UPDATE time_entries
		SET is_deleted = 1, updated_at = ?
		WHERE id = ?
	`

	result, err := tx.ExecContext(ctx, query, formatTime(), id)
	if err != nil {
		return fmt.Errorf("failed to delete time entry: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("time entry not found")
	}

	// Create audit record
	historyQuery := `
		INSERT INTO entry_history (entry_id, field_name, old_value, new_value, change_reason, changed_at)
		VALUES (?, 'is_deleted', '0', '1', ?, ?)
	`

	_, err = tx.ExecContext(ctx, historyQuery, id, reason, formatTime())
	if err != nil {
		return fmt.Errorf("failed to create audit record: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// List retrieves time entries with optional filters
func (r *EntryRepo) List(ctx context.Context, clientID *int64, start, end *time.Time, includeLocked bool) ([]*domain.TimeEntry, error) {
	query := `
		SELECT id, client_id, description, start_time, end_time, duration_seconds,
		       hourly_rate, is_billable, is_deleted, invoice_id, created_at, updated_at
		FROM time_entries
		WHERE is_deleted = 0
	`
	args := make([]interface{}, 0)

	if clientID != nil {
		query += " AND client_id = ?"
		args = append(args, *clientID)
	}

	if start != nil {
		query += " AND start_time >= ?"
		args = append(args, start.Format(timeLayout))
	}

	if end != nil {
		query += " AND start_time <= ?"
		args = append(args, end.Format(timeLayout))
	}

	if !includeLocked {
		query += " AND invoice_id IS NULL"
	}

	query += " ORDER BY start_time DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list time entries: %w", err)
	}
	defer rows.Close()

	entries := make([]*domain.TimeEntry, 0)
	for rows.Next() {
		entry := &domain.TimeEntry{}
		var startTime, createdAt, updatedAt sql.NullString
		var endTime, durationSeconds, invoiceID sql.NullString

		err := rows.Scan(
			&entry.ID,
			&entry.ClientID,
			&entry.Description,
			&startTime,
			&endTime,
			&durationSeconds,
			&entry.HourlyRate,
			&entry.IsBillable,
			&entry.IsDeleted,
			&invoiceID,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan time entry: %w", err)
		}

		if err := scanTimeEntry(entry, startTime, endTime, durationSeconds, invoiceID, createdAt, updatedAt); err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating time entries: %w", err)
	}

	return entries, nil
}

// GetUnbilledByClient retrieves unbilled time entries for a client within a date range
func (r *EntryRepo) GetUnbilledByClient(ctx context.Context, clientID int64, start, end time.Time) ([]*domain.TimeEntry, error) {
	query := `
		SELECT id, client_id, description, start_time, end_time, duration_seconds,
		       hourly_rate, is_billable, is_deleted, invoice_id, created_at, updated_at
		FROM time_entries
		WHERE client_id = ?
		  AND invoice_id IS NULL
		  AND is_deleted = 0
		  AND start_time >= ?
		  AND start_time <= ?
		  AND end_time IS NOT NULL
		ORDER BY start_time
	`

	rows, err := r.db.QueryContext(ctx, query, clientID, start.Format(timeLayout), end.Format(timeLayout))
	if err != nil {
		return nil, fmt.Errorf("failed to get unbilled entries: %w", err)
	}
	defer rows.Close()

	entries := make([]*domain.TimeEntry, 0)
	for rows.Next() {
		entry := &domain.TimeEntry{}
		var startTime, createdAt, updatedAt sql.NullString
		var endTime, durationSeconds, invoiceID sql.NullString

		err := rows.Scan(
			&entry.ID,
			&entry.ClientID,
			&entry.Description,
			&startTime,
			&endTime,
			&durationSeconds,
			&entry.HourlyRate,
			&entry.IsBillable,
			&entry.IsDeleted,
			&invoiceID,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan time entry: %w", err)
		}

		if err := scanTimeEntry(entry, startTime, endTime, durationSeconds, invoiceID, createdAt, updatedAt); err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating unbilled entries: %w", err)
	}

	return entries, nil
}

// IsLocked checks if a time entry is locked (attached to an invoice)
func (r *EntryRepo) IsLocked(ctx context.Context, id int64) (bool, error) {
	var invoiceID sql.NullInt64
	query := "SELECT invoice_id FROM time_entries WHERE id = ?"

	err := r.db.QueryRowContext(ctx, query, id).Scan(&invoiceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("time entry not found")
		}
		return false, fmt.Errorf("failed to check lock status: %w", err)
	}

	return invoiceID.Valid, nil
}

// LockForInvoice locks multiple time entries by attaching them to an invoice
func (r *EntryRepo) LockForInvoice(ctx context.Context, entryIDs []int64, invoiceID int64) error {
	if len(entryIDs) == 0 {
		return nil
	}

	// Begin transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statement for efficiency
	stmt, err := tx.PrepareContext(ctx, `
		UPDATE time_entries
		SET invoice_id = ?, updated_at = ?
		WHERE id = ? AND invoice_id IS NULL AND is_deleted = 0
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	updateTime := formatTime()
	for _, entryID := range entryIDs {
		result, err := stmt.ExecContext(ctx, invoiceID, updateTime, entryID)
		if err != nil {
			return fmt.Errorf("failed to lock entry %d: %w", entryID, err)
		}

		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected for entry %d: %w", entryID, err)
		}
		if rows == 0 {
			return fmt.Errorf("entry %d not found, already locked, or deleted", entryID)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetHistory retrieves the audit trail for a time entry
func (r *EntryRepo) GetHistory(ctx context.Context, entryID int64) ([]*domain.EntryHistory, error) {
	query := `
		SELECT id, entry_id, field_name, old_value, new_value, change_reason, changed_at
		FROM entry_history
		WHERE entry_id = ?
		ORDER BY changed_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, entryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entry history: %w", err)
	}
	defer rows.Close()

	history := make([]*domain.EntryHistory, 0)
	for rows.Next() {
		h := &domain.EntryHistory{}
		var changedAt string

		err := rows.Scan(
			&h.ID,
			&h.EntryID,
			&h.FieldName,
			&h.OldValue,
			&h.NewValue,
			&h.ChangeReason,
			&changedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan history: %w", err)
		}

		if h.ChangedAt, err = parseTime(changedAt); err != nil {
			return nil, fmt.Errorf("failed to parse changed_at: %w", err)
		}

		history = append(history, h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating history: %w", err)
	}

	return history, nil
}

// createAuditRecords creates history records for changed fields
func (r *EntryRepo) createAuditRecords(ctx context.Context, tx *sql.Tx, old, new *domain.TimeEntry, reason string) error {
	changedAt := formatTime()

	// Helper to insert audit record
	insertHistory := func(fieldName, oldVal, newVal string) error {
		if oldVal == newVal {
			return nil
		}
		query := `
			INSERT INTO entry_history (entry_id, field_name, old_value, new_value, change_reason, changed_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`
		_, err := tx.ExecContext(ctx, query, new.ID, fieldName, oldVal, newVal, reason, changedAt)
		return err
	}

	// Check each field for changes
	if old.ClientID != new.ClientID {
		if err := insertHistory("client_id", strconv.FormatInt(old.ClientID, 10), strconv.FormatInt(new.ClientID, 10)); err != nil {
			return fmt.Errorf("failed to audit client_id change: %w", err)
		}
	}

	if old.Description != new.Description {
		if err := insertHistory("description", old.Description, new.Description); err != nil {
			return fmt.Errorf("failed to audit description change: %w", err)
		}
	}

	if !old.StartTime.Equal(new.StartTime) {
		if err := insertHistory("start_time", old.StartTime.Format(timeLayout), new.StartTime.Format(timeLayout)); err != nil {
			return fmt.Errorf("failed to audit start_time change: %w", err)
		}
	}

	oldEnd := ""
	newEnd := ""
	if old.EndTime != nil {
		oldEnd = old.EndTime.Format(timeLayout)
	}
	if new.EndTime != nil {
		newEnd = new.EndTime.Format(timeLayout)
	}
	if oldEnd != newEnd {
		if err := insertHistory("end_time", oldEnd, newEnd); err != nil {
			return fmt.Errorf("failed to audit end_time change: %w", err)
		}
	}

	if old.HourlyRate != new.HourlyRate {
		if err := insertHistory("hourly_rate", fmt.Sprintf("%.2f", old.HourlyRate), fmt.Sprintf("%.2f", new.HourlyRate)); err != nil {
			return fmt.Errorf("failed to audit hourly_rate change: %w", err)
		}
	}

	if old.IsBillable != new.IsBillable {
		if err := insertHistory("is_billable", strconv.FormatBool(old.IsBillable), strconv.FormatBool(new.IsBillable)); err != nil {
			return fmt.Errorf("failed to audit is_billable change: %w", err)
		}
	}

	return nil
}

// scanTimeEntry is a helper to parse time entry fields
func scanTimeEntry(entry *domain.TimeEntry, startTime, endTime, durationSeconds, invoiceID, createdAt, updatedAt sql.NullString) error {
	var err error

	if entry.StartTime, err = parseTime(startTime.String); err != nil {
		return fmt.Errorf("failed to parse start_time: %w", err)
	}

	if endTime.Valid {
		t, err := parseTime(endTime.String)
		if err != nil {
			return fmt.Errorf("failed to parse end_time: %w", err)
		}
		entry.EndTime = &t
	}

	if durationSeconds.Valid {
		val, err := strconv.ParseInt(durationSeconds.String, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse duration_seconds: %w", err)
		}
		entry.DurationSeconds = &val
	}

	if invoiceID.Valid {
		val, err := strconv.ParseInt(invoiceID.String, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse invoice_id: %w", err)
		}
		entry.InvoiceID = &val
	}

	if entry.CreatedAt, err = parseTime(createdAt.String); err != nil {
		return fmt.Errorf("failed to parse created_at: %w", err)
	}

	if entry.UpdatedAt, err = parseTime(updatedAt.String); err != nil {
		return fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return nil
}
