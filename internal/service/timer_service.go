package service

import (
	"context"
	"errors"
	"time"

	"github.com/andy/timesink/internal/domain"
	"github.com/andy/timesink/internal/repository"
)

var (
	ErrTimerAlreadyRunning = errors.New("timer is already running")
	ErrTimerNotRunning     = errors.New("timer is not running")
	ErrTimerNotPaused      = errors.New("timer is not paused")
	ErrNoActiveTimer       = errors.New("no active timer")
)

// TimerService manages the timer state machine
type TimerService interface {
	// GetState returns the current timer state (idle, running, paused)
	GetState(ctx context.Context) (domain.TimerState, error)

	// GetActiveTimer returns the current active timer, or nil if idle
	GetActiveTimer(ctx context.Context) (*domain.ActiveTimer, error)

	// Start creates a new timer (only from Idle state)
	Start(ctx context.Context, clientID int64, description string) error

	// Pause pauses the running timer (only from Running state)
	Pause(ctx context.Context) error

	// Resume resumes a paused timer (only from Paused state)
	Resume(ctx context.Context) error

	// Stop stops the timer and creates a time entry (from Running or Paused)
	Stop(ctx context.Context) (*domain.TimeEntry, error)

	// Discard discards the active timer without creating an entry
	Discard(ctx context.Context) error

	// ElapsedDuration returns the elapsed time of the active timer
	ElapsedDuration(ctx context.Context) (time.Duration, error)

	// AccruedValue calculates the current value of the active timer
	AccruedValue(ctx context.Context, hourlyRate float64) (float64, error)

	// RecoverFromCrash checks for an existing timer on startup
	RecoverFromCrash(ctx context.Context) error
}

type timerService struct {
	timerRepo  repository.TimerRepository
	entryRepo  repository.TimeEntryRepository
	clientRepo repository.ClientRepository
}

// NewTimerService creates a new timer service
func NewTimerService(
	timerRepo repository.TimerRepository,
	entryRepo repository.TimeEntryRepository,
	clientRepo repository.ClientRepository,
) TimerService {
	return &timerService{
		timerRepo:  timerRepo,
		entryRepo:  entryRepo,
		clientRepo: clientRepo,
	}
}

func (s *timerService) GetState(ctx context.Context) (domain.TimerState, error) {
	timer, err := s.timerRepo.Get(ctx)
	if err != nil {
		return "", err
	}
	if timer == nil {
		return domain.TimerStateIdle, nil
	}
	return timer.State(), nil
}

func (s *timerService) GetActiveTimer(ctx context.Context) (*domain.ActiveTimer, error) {
	return s.timerRepo.Get(ctx)
}

func (s *timerService) Start(ctx context.Context, clientID int64, description string) error {
	// Verify client exists
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return err
	}
	if client == nil {
		return errors.New("client not found")
	}

	// Check no timer is already running
	existingTimer, err := s.timerRepo.Get(ctx)
	if err != nil {
		return err
	}
	if existingTimer != nil {
		return ErrTimerAlreadyRunning
	}

	// Create and save new timer
	timer := domain.NewActiveTimer(clientID, description)
	return s.timerRepo.Save(ctx, timer)
}

func (s *timerService) Pause(ctx context.Context) error {
	timer, err := s.timerRepo.Get(ctx)
	if err != nil {
		return err
	}
	if timer == nil {
		return ErrNoActiveTimer
	}

	state := timer.State()
	if state != domain.TimerStateRunning {
		return ErrTimerNotRunning
	}

	timer.Pause()
	return s.timerRepo.Save(ctx, timer)
}

func (s *timerService) Resume(ctx context.Context) error {
	timer, err := s.timerRepo.Get(ctx)
	if err != nil {
		return err
	}
	if timer == nil {
		return ErrNoActiveTimer
	}

	state := timer.State()
	if state != domain.TimerStatePaused {
		return ErrTimerNotPaused
	}

	timer.Resume()
	return s.timerRepo.Save(ctx, timer)
}

func (s *timerService) Stop(ctx context.Context) (*domain.TimeEntry, error) {
	timer, err := s.timerRepo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if timer == nil {
		return nil, ErrNoActiveTimer
	}

	// Get client to retrieve hourly rate
	client, err := s.clientRepo.GetByID(ctx, timer.ClientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, errors.New("client not found")
	}

	// Convert timer to time entry
	entry := timer.ToTimeEntry(client.HourlyRate)

	// Save entry
	if err := s.entryRepo.Create(ctx, entry); err != nil {
		return nil, err
	}

	// Delete active timer
	if err := s.timerRepo.Delete(ctx); err != nil {
		return nil, err
	}

	return entry, nil
}

func (s *timerService) Discard(ctx context.Context) error {
	timer, err := s.timerRepo.Get(ctx)
	if err != nil {
		return err
	}
	if timer == nil {
		return ErrNoActiveTimer
	}

	return s.timerRepo.Delete(ctx)
}

func (s *timerService) ElapsedDuration(ctx context.Context) (time.Duration, error) {
	timer, err := s.timerRepo.Get(ctx)
	if err != nil {
		return 0, err
	}
	if timer == nil {
		return 0, ErrNoActiveTimer
	}

	return timer.Elapsed(), nil
}

func (s *timerService) AccruedValue(ctx context.Context, hourlyRate float64) (float64, error) {
	elapsed, err := s.ElapsedDuration(ctx)
	if err != nil {
		return 0, err
	}

	hours := elapsed.Hours()
	return hours * hourlyRate, nil
}

func (s *timerService) RecoverFromCrash(ctx context.Context) error {
	timer, err := s.timerRepo.Get(ctx)
	if err != nil {
		return err
	}

	// If timer exists, it was running before crash - no action needed
	// The timer repository persists the state, so it will continue
	if timer != nil {
		// Could log a message here about recovered timer
		return nil
	}

	return nil
}
