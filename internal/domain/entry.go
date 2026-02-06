package domain

import (
	"errors"
	"time"
)

type TimeEntry struct {
	ID              int64
	ClientID        int64
	Description     string
	StartTime       time.Time
	EndTime         *time.Time // nil if still running
	DurationSeconds *int64     // calculated, nil if still running
	HourlyRate      float64    // frozen at entry time
	IsBillable      bool
	IsDeleted       bool   // soft delete
	InvoiceID       *int64 // nil = unbilled, non-nil = locked
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewTimeEntry creates a new time entry
func NewTimeEntry(clientID int64, description string, hourlyRate float64) *TimeEntry {
	now := time.Now()
	return &TimeEntry{
		ClientID:    clientID,
		Description: description,
		StartTime:   now,
		HourlyRate:  hourlyRate,
		IsBillable:  true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Duration returns the duration of the entry
func (e *TimeEntry) Duration() time.Duration {
	if e.EndTime == nil {
		return time.Since(e.StartTime)
	}
	return e.EndTime.Sub(e.StartTime)
}

// Amount returns the billable amount (hours * rate)
func (e *TimeEntry) Amount() float64 {
	if !e.IsBillable {
		return 0
	}
	hours := e.Duration().Hours()
	return hours * e.HourlyRate
}

// IsLocked returns true if the entry is attached to an invoice
func (e *TimeEntry) IsLocked() bool {
	return e.InvoiceID != nil
}

// IsRunning returns true if the entry has no end time
func (e *TimeEntry) IsRunning() bool {
	return e.EndTime == nil
}

// Stop sets the end time and calculates duration
func (e *TimeEntry) Stop(endTime time.Time) {
	e.EndTime = &endTime
	durationSecs := int64(e.Duration().Seconds())
	e.DurationSeconds = &durationSecs
	e.UpdatedAt = time.Now()
}

// Validate returns an error if the entry is invalid
func (e *TimeEntry) Validate() error {
	if e.ClientID <= 0 {
		return errors.New("client ID is required")
	}
	if e.HourlyRate < 0 {
		return errors.New("hourly rate cannot be negative")
	}
	if e.StartTime.IsZero() {
		return errors.New("start time is required")
	}
	if e.EndTime != nil && e.EndTime.Before(e.StartTime) {
		return errors.New("end time must be after start time")
	}
	return nil
}
