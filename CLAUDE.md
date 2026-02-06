# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

timesink is a CLI time tracking tool for freelancers written in Go. It features a keyboard-driven TUI for managing clients, timesheets, and invoices. The project is in early development - mockups exist but code implementation has not started.

## Build Commands

```bash
# Build
go build -o timesink ./cmd/timesink

# Run tests
go test ./...

# Run single test
go test ./internal/db -run TestClientCreate

# Install locally
go install ./cmd/timesink
```

## Architecture

### Planned Structure
```
cmd/timesink/main.go    # Entry point, CLI command handling
internal/
  db/                   # SQLite persistence layer
  models/               # Domain: Client, TimeEntry, Invoice
  tui/                  # Terminal UI (keyboard-driven navigation)
  timer/                # Timer state machine (start/stop/pause)
  invoice/              # Invoice generation and PDF export
  reports/              # Analytics and summaries
```

### Data Flow
- SQLite database at `~/.config/timesink/timesink.db`
- Config file at `~/.config/timesink/config.yaml`
- Generated invoices at `~/.config/timesink/invoices/`

### Key Domain Models
- **Client**: Name, hourly rate, contact info, active/archived status
- **TimeEntry**: Start/end timestamps, client reference, task description, billable flag, rate at time of entry
- **Invoice**: Invoice number, client ref, date range, line items from time entries, status (draft/sent/paid/overdue)

### TUI Navigation
The app uses single-key navigation from a main menu:
- `T` Timer, `E` Entries, `C` Clients, `I` Invoices, `R` Reports, `Q` Quit
- Each view supports arrow navigation, Enter for details, and contextual hotkeys

See `mockups/` directory for complete ASCII UI designs.