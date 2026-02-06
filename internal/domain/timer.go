package domain

import "time"

type TimerState string

const (
	TimerStateIdle    TimerState = "idle"
	TimerStateRunning TimerState = "running"
	TimerStatePaused  TimerState = "paused"
)

type ActiveTimer struct {
	ClientID           int64
	Description        string
	StartTime          time.Time
	PausedAt           *time.Time
	TotalPausedSeconds int64
}

// NewActiveTimer creates a new running timer
func NewActiveTimer(clientID int64, description string) *ActiveTimer {
	return &ActiveTimer{
		ClientID:    clientID,
		Description: description,
		StartTime:   time.Now(),
	}
}

// State returns the current timer state
func (t *ActiveTimer) State() TimerState {
	if t.PausedAt != nil {
		return TimerStatePaused
	}
	return TimerStateRunning
}

// Elapsed returns the active duration (excluding paused time)
func (t *ActiveTimer) Elapsed() time.Duration {
	totalElapsed := time.Since(t.StartTime)
	pausedDuration := time.Duration(t.TotalPausedSeconds) * time.Second

	// If currently paused, add current pause duration
	if t.PausedAt != nil {
		pausedDuration += time.Since(*t.PausedAt)
	}

	return totalElapsed - pausedDuration
}

// Pause pauses the timer
func (t *ActiveTimer) Pause() {
	if t.PausedAt == nil {
		now := time.Now()
		t.PausedAt = &now
	}
}

// Resume resumes a paused timer
func (t *ActiveTimer) Resume() {
	if t.PausedAt != nil {
		pauseDuration := time.Since(*t.PausedAt)
		t.TotalPausedSeconds += int64(pauseDuration.Seconds())
		t.PausedAt = nil
	}
}

// ToTimeEntry converts the timer to a time entry when stopped
func (t *ActiveTimer) ToTimeEntry(hourlyRate float64) *TimeEntry {
	// If paused, finalize the pause duration
	if t.PausedAt != nil {
		t.Resume()
	}

	now := time.Now()
	durationSecs := int64(t.Elapsed().Seconds())

	return &TimeEntry{
		ClientID:        t.ClientID,
		Description:     t.Description,
		StartTime:       t.StartTime,
		EndTime:         &now,
		DurationSeconds: &durationSecs,
		HourlyRate:      hourlyRate,
		IsBillable:      true,
		CreatedAt:       t.StartTime,
		UpdatedAt:       now,
	}
}
