package app

import (
	"context"
	"fmt"
	"syscall"

	"github.com/andy/timesink/internal/config"
	"github.com/andy/timesink/internal/crypto"
	"github.com/andy/timesink/internal/db"
	"github.com/andy/timesink/internal/repository"
	"github.com/andy/timesink/internal/service"
	"golang.org/x/term"
)

// App is the dependency injection container for all application components
type App struct {
	Config *config.Config
	DB     *db.DB

	// Repositories
	ClientRepo  repository.ClientRepository
	EntryRepo   repository.TimeEntryRepository
	InvoiceRepo repository.InvoiceRepository
	TimerRepo   repository.TimerRepository

	// Services
	TimerService   service.TimerService
	InvoiceService service.InvoiceService
	ReportService  service.ReportService
}

// New creates a new App instance, initializing all dependencies
// It handles:
// 1. Loading config
// 2. Getting encryption key from keyring
// 3. Opening database
// 4. Running migrations
// 5. Creating repositories
// 6. Creating services
func New(ctx context.Context) (*App, error) {
	// Load config from default path
	cfg, err := config.LoadDefault()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return NewWithConfig(ctx, cfg)
}

// NewWithConfig creates an App with a provided config (useful for testing)
func NewWithConfig(ctx context.Context, cfg *config.Config) (*App, error) {
	// Ensure all necessary directories exist
	if err := cfg.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	// Get keyring for secure password storage
	keyring := crypto.NewKeyring()

	// Try to get existing encryption key
	password, err := keyring.GetKey()
	if err != nil {
		// No key exists, prompt user to set one
		fmt.Println("Setting up database encryption for the first time...")
		password, err = promptForPassword()
		if err != nil {
			return nil, fmt.Errorf("failed to set password: %w", err)
		}

		// Store the key in keyring
		if err := keyring.SetKey(password); err != nil {
			return nil, fmt.Errorf("failed to store encryption key: %w", err)
		}
	}

	// Open the database with encryption
	database, err := db.Open(cfg.Database.Path, password)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Run migrations to ensure schema is up to date
	if err := database.RunMigrations(); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create repositories
	clientRepo := repository.NewClientRepo(database)
	entryRepo := repository.NewEntryRepo(database)
	invoiceRepo := repository.NewInvoiceRepo(database)
	timerRepo := repository.NewTimerRepo(database)

	// Create services with their dependencies
	timerService := service.NewTimerService(timerRepo, entryRepo, clientRepo)
	invoiceService := service.NewInvoiceService(invoiceRepo, entryRepo, clientRepo)
	reportService := service.NewReportService(entryRepo, invoiceRepo)

	return &App{
		Config:         cfg,
		DB:             database,
		ClientRepo:     clientRepo,
		EntryRepo:      entryRepo,
		InvoiceRepo:    invoiceRepo,
		TimerRepo:      timerRepo,
		TimerService:   timerService,
		InvoiceService: invoiceService,
		ReportService:  reportService,
	}, nil
}

// Close cleanly shuts down the application
func (a *App) Close() error {
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}

// promptForPassword prompts user for a new database password (first run)
// This should be called when keyring has no stored key
func promptForPassword() (string, error) {
	fmt.Println()
	fmt.Println("Your time tracking data will be encrypted with a password.")
	fmt.Println("This password will be stored securely in your system keyring.")
	fmt.Println()
	fmt.Print("Enter a password for database encryption: ")

	// Read password securely (no echo)
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // New line after password input
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	if len(password) == 0 {
		return "", fmt.Errorf("password cannot be empty")
	}

	// Confirm password
	fmt.Print("Confirm password: ")
	confirm, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // New line after confirmation
	if err != nil {
		return "", fmt.Errorf("failed to read confirmation: %w", err)
	}

	// Check if passwords match
	if string(password) != string(confirm) {
		return "", fmt.Errorf("passwords do not match")
	}

	fmt.Println()
	fmt.Println("✓ Database encryption configured successfully")
	fmt.Println()

	return string(password), nil
}

// RecoverTimer checks for an existing timer on startup
// This is useful for crash recovery to let the user know about an active timer
func (a *App) RecoverTimer(ctx context.Context) error {
	return a.TimerService.RecoverFromCrash(ctx)
}

// SaveConfig saves the current configuration to disk
func (a *App) SaveConfig() error {
	return a.Config.Save(config.DefaultConfigPath())
}
