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

// TimerRepo is a SQLite implementation of TimerRepository
type TimerRepo struct {
	db *db.DB
}

// NewTimerRepo creates a new TimerRepo
func NewTimerRepo(database *db.DB) *TimerRepo {
	return &TimerRepo{db: database}
}

// Get retrieves the active timer, or returns nil if no timer is running
func (r *TimerRepo) Get(ctx context.Context) (*domain.ActiveTimer, error) {
	query := `
		SELECT client_id, description, start_time, paused_at, total_paused_seconds
		FROM active_timer
		WHERE id = 1
	`

	timer := &domain.ActiveTimer{}
	var startTime string
	var pausedAt sql.NullString

	err := r.db.QueryRowContext(ctx, query).Scan(
		&timer.ClientID,
		&timer.Description,
		&startTime,
		&pausedAt,
		&timer.TotalPausedSeconds,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No active timer
		}
		return nil, fmt.Errorf("failed to get active timer: %w", err)
	}

	if timer.StartTime, err = parseTime(startTime); err != nil {
		return nil, fmt.Errorf("failed to parse start_time: %w", err)
	}

	if pausedAt.Valid {
		t, err := parseTime(pausedAt.String)
		if err != nil {
			return nil, fmt.Errorf("failed to parse paused_at: %w", err)
		}
		timer.PausedAt = &t
	}

	return timer, nil
}

// Save saves the active timer (insert or replace)
func (r *TimerRepo) Save(ctx context.Context, timer *domain.ActiveTimer) error {
	query := `
		INSERT OR REPLACE INTO active_timer (id, client_id, description, start_time, paused_at, total_paused_seconds)
		VALUES (1, ?, ?, ?, ?, ?)
	`

	var pausedAt interface{}
	if timer.PausedAt != nil {
		pausedAt = timer.PausedAt.Format(timeLayout)
	}

	_, err := r.db.ExecContext(ctx, query,
		timer.ClientID,
		timer.Description,
		timer.StartTime.Format(timeLayout),
		pausedAt,
		timer.TotalPausedSeconds,
	)
	if err != nil {
		return fmt.Errorf("failed to save active timer: %w", err)
	}

	return nil
}

// Delete removes the active timer
func (r *TimerRepo) Delete(ctx context.Context) error {
	query := "DELETE FROM active_timer WHERE id = 1"

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to delete active timer: %w", err)
	}

	return nil
}
