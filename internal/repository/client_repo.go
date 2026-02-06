package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/andy/timesink/internal/db"
	"github.com/andy/timesink/internal/domain"
)

// ClientRepo is a SQLite implementation of ClientRepository
type ClientRepo struct {
	db *db.DB
}

// NewClientRepo creates a new ClientRepo
func NewClientRepo(database *db.DB) *ClientRepo {
	return &ClientRepo{db: database}
}

// Create inserts a new client into the database
func (r *ClientRepo) Create(ctx context.Context, client *domain.Client) error {
	if err := client.Validate(); err != nil {
		return fmt.Errorf("invalid client: %w", err)
	}

	query := `
		INSERT INTO clients (name, email, hourly_rate, notes, is_archived, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query,
		client.Name,
		client.Email,
		client.HourlyRate,
		client.Notes,
		client.IsArchived,
		client.CreatedAt.Format(timeLayout),
		client.UpdatedAt.Format(timeLayout),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get client ID: %w", err)
	}

	client.ID = id
	return nil
}

// GetByID retrieves a client by ID
func (r *ClientRepo) GetByID(ctx context.Context, id int64) (*domain.Client, error) {
	query := `
		SELECT id, name, email, hourly_rate, notes, is_archived, created_at, updated_at
		FROM clients
		WHERE id = ?
	`

	client := &domain.Client{}
	var createdAt, updatedAt string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&client.ID,
		&client.Name,
		&client.Email,
		&client.HourlyRate,
		&client.Notes,
		&client.IsArchived,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("client not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	if client.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}
	if client.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return client, nil
}

// GetByName retrieves a client by name
func (r *ClientRepo) GetByName(ctx context.Context, name string) (*domain.Client, error) {
	query := `
		SELECT id, name, email, hourly_rate, notes, is_archived, created_at, updated_at
		FROM clients
		WHERE name = ?
	`

	client := &domain.Client{}
	var createdAt, updatedAt string

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&client.ID,
		&client.Name,
		&client.Email,
		&client.HourlyRate,
		&client.Notes,
		&client.IsArchived,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("client not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	if client.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}
	if client.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return client, nil
}

// List retrieves all clients, optionally including archived ones
func (r *ClientRepo) List(ctx context.Context, includeArchived bool) ([]*domain.Client, error) {
	query := `
		SELECT id, name, email, hourly_rate, notes, is_archived, created_at, updated_at
		FROM clients
		WHERE is_archived = 0 OR ? = 1
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query, includeArchived)
	if err != nil {
		return nil, fmt.Errorf("failed to list clients: %w", err)
	}
	defer rows.Close()

	clients := make([]*domain.Client, 0)
	for rows.Next() {
		client := &domain.Client{}
		var createdAt, updatedAt string

		err := rows.Scan(
			&client.ID,
			&client.Name,
			&client.Email,
			&client.HourlyRate,
			&client.Notes,
			&client.IsArchived,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan client: %w", err)
		}

		if client.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		if client.UpdatedAt, err = parseTime(updatedAt); err != nil {
			return nil, fmt.Errorf("failed to parse updated_at: %w", err)
		}

		clients = append(clients, client)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating clients: %w", err)
	}

	return clients, nil
}

// Update updates an existing client
func (r *ClientRepo) Update(ctx context.Context, client *domain.Client) error {
	if err := client.Validate(); err != nil {
		return fmt.Errorf("invalid client: %w", err)
	}

	client.UpdatedAt = client.UpdatedAt // Keep the passed-in time or use time.Now()

	query := `
		UPDATE clients
		SET name = ?, email = ?, hourly_rate = ?, notes = ?, is_archived = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		client.Name,
		client.Email,
		client.HourlyRate,
		client.Notes,
		client.IsArchived,
		client.UpdatedAt.Format(timeLayout),
		client.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update client: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}

// Archive marks a client as archived
func (r *ClientRepo) Archive(ctx context.Context, id int64) error {
	query := `
		UPDATE clients
		SET is_archived = 1, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, formatTime(), id)
	if err != nil {
		return fmt.Errorf("failed to archive client: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}

// Unarchive marks a client as active
func (r *ClientRepo) Unarchive(ctx context.Context, id int64) error {
	query := `
		UPDATE clients
		SET is_archived = 0, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, formatTime(), id)
	if err != nil {
		return fmt.Errorf("failed to unarchive client: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}
