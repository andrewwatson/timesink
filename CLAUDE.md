# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

timesink is a CLI time tracking tool for freelancers written in Go. It has a keyboard-driven TUI (Bubble Tea) for managing clients, tracking time, generating invoices, and viewing reports. Data is stored in an encrypted SQLite database (SQLCipher) with the key in the system keyring.

## Build Commands

```bash
go build ./...              # Build all packages
go build -o timesink ./cmd/timesink  # Build binary
go test ./...               # Run tests
```

## Architecture

### Project Structure
```
cmd/timesink/main.go        # Entry point, initializes app then runs CLI
internal/
  app/app.go                # Dependency injection container (App struct)
  config/config.go          # YAML config loading/saving (~/.config/timesink/config.yaml)
  crypto/                   # Keyring-based encryption key storage
  db/
    db.go                   # SQLCipher database connection
    migrations.go           # Schema migrations (clients, time_entries, invoices, etc.)
  domain/                   # Domain models: Client, TimeEntry, Invoice, Timer
  repository/               # SQLite repositories implementing interfaces
    repository.go           # Repository interfaces (ClientRepository, TimeEntryRepository, etc.)
    client_repo.go
    entry_repo.go
    invoice_repo.go
    timer_repo.go
  service/                  # Business logic layer
    timer_service.go        # Timer lifecycle (start/stop/pause/resume)
    invoice_service.go      # Invoice lifecycle (draft/finalize/send/pay)
    report_service.go       # Weekly/monthly/daily summaries
  cli/                      # Cobra commands
    root.go                 # Root command, registers subcommands
    timer.go, clients.go, entries.go, invoices.go, reset.go
    tui.go                  # Launches TUI
  tui/                      # Bubble Tea TUI
    model.go                # Root model, screen routing, InputCapturer interface
    keys.go                 # Key bindings (DefaultKeyMap)
    styles.go               # Lipgloss color palette and styles
    messages.go             # Shared messages (RefreshDataMsg, SwitchScreenMsg, ErrorMsg)
    helpers.go              # formatHours, formatMoney, truncateStr
    dashboard_screen.go
    timer_screen.go
    entries_screen.go       # List + manual entry form with client picker
    clients_screen.go       # List + create/edit form (reference pattern for forms)
    invoices_screen.go      # List + detail + multi-step invoice generation flow
    reports_screen.go       # Weekly chart, daily detail, client breakdown, monthly revenue
    settings_screen.go      # View/edit invoice config settings
```

### Key Patterns

**Screen lifecycle**: Screens are lazy-initialized via `initScreen()`. First visit calls `Init()`, subsequent visits send `RefreshDataMsg` to reload data.

**InputCapturer interface**: Screens with text input forms implement `IsCapturingInput() bool`. When true, global nav keys (T/E/C/I/R/S/Q) are suppressed so they go to the form instead.

**Form pattern** (see clients_screen.go as reference):
- Mode enum (list/new/edit)
- `[]textinput.Model` fields with `fieldFocus` index
- Tab/Shift+Tab to navigate, Ctrl+S to save, Esc to cancel
- Async save via `tea.Cmd` returning a result message
- Route ALL messages to `updateForm()` when in form mode (before the `tea.KeyMsg` switch)

**Async operations**: Heavy work (DB queries, invoice generation) runs in `tea.Cmd` functions that return typed messages. Never block the Update loop.

**Message routing**: The root `Model.Update()` handles global keys, then routes to the active screen's `Update()`. Each screen handles its own message types.

### Data Flow
- Encrypted SQLite database at path from config (default: `~/.config/timesink/timesink.db`)
- Config file at `~/.config/timesink/config.yaml`
- Invoice .txt files exported to configurable directory (default: current working directory)
- Encryption password stored in system keyring (macOS Keychain)

### TUI Navigation
Single-key global navigation: `T` Timer, `E` Entries, `C` Clients, `I` Invoices, `R` Reports, `S` Settings, `Q` Quit

### Domain Models
- **Client**: Name, hourly rate, email, notes, active/archived status
- **TimeEntry**: Start/end timestamps, client ID, description, hourly rate (frozen at entry time), billable flag, invoice_id (nil=unbilled, non-nil=locked)
- **Invoice**: Invoice number (auto-generated), client ID, period start/end, status (draft/finalized/sent/paid/overdue), subtotal/tax/total, due date
- **InvoiceLineItem**: Links invoice to entry with date, description, hours, rate, amount
- **Timer** (active_timer table): At most one active timer, with start time, pause state, client/description

### CLI Commands
Root command launches TUI by default. Subcommands: `timer`, `clients`, `entries`, `invoices`, `reset`, `tui`.
