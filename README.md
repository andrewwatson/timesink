# timesink

> A delightfully ironic CLI time tracker for freelancers and contractors who bill by the hour

`timesink` is a terminal-based time tracking utility with a keyboard-driven TUI for managing clients, timesheets, and invoices. Because tracking your time shouldn't waste your time.

## Features

- **Live timers** - Start/stop/pause timers with a single keystroke
- **Client management** - Track multiple clients with custom hourly rates
- **Time entries** - Manual entry and editing of time logs
- **Invoice generation** - Create professional invoices from time entries
- **Reports & statistics** - Weekly/monthly summaries and billable analytics
- **Fully keyboard-driven** - No mouse required, navigate everything with hotkeys
- **ASCII art TUI** - Because we're professionals with a sense of humor

## Screenshots

```
╔══════════════════════════════════════════════════════════════╗
║ timesink v0.1.0                    Week of Feb 3-9, 2026     ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║  This Week:  32.5 hrs        Billable:  $4,875.00          ║
║  Today:       4.0 hrs        Outstanding: $12,340.00        ║
║                                                              ║
║  Active Timers                                               ║
║  ● Cox Automotive - Security Audit        [02:34:12]        ║
║                                                              ║
╠══════════════════════════════════════════════════════════════╣
║ [T]imer  [E]ntries  [C]lients  [I]nvoices  [R]eports  [Q]uit║
╚══════════════════════════════════════════════════════════════╝
```

See the [mockups directory](./mockups) for full UI designs.

## Installation

```bash
# From source
git clone https://github.com/andrewwatson/timesink.git
cd timesink
go build -o timesink ./cmd/timesink

# Install globally
go install github.com/andrewwatson/timesink/cmd/timesink@latest
```

## Quick Start

```bash
# Start timesink
timesink

# Or start a timer directly from the command line
timesink start "Client Name" "Task description"

# Stop the active timer
timesink stop

# View today's time entries
timesink today

# Generate an invoice
timesink invoice create --client "Client Name" --period "2026-02"
```

## Usage

### Interactive TUI Mode

Just run `timesink` with no arguments to launch the interactive interface:

```bash
timesink
```

**Main Menu Hotkeys:**
- `T` - Timer management
- `E` - View/edit time entries
- `C` - Client management
- `I` - Invoices
- `R` - Reports and statistics
- `Q` - Quit

### Command Line Mode

For quick operations without launching the full TUI:

```bash
# Timer operations
timesink start <client> [description]
timesink stop
timesink pause
timesink resume
timesink status

# Time entries
timesink log <client> <hours> [description]
timesink today
timesink week
timesink month

# Client management
timesink client add <name> --rate <hourly_rate>
timesink client list
timesink client edit <name>

# Invoicing
timesink invoice create --client <name> --start <date> --end <date>
timesink invoice list
timesink invoice export <invoice_id> --format pdf

# Reports
timesink report weekly
timesink report monthly
timesink report client <name>
```

## Configuration

`timesink` stores its data in `~/.config/timesink/`:

```
~/.config/timesink/
├── config.yaml       # User preferences
├── timesink.db      # SQLite database with all data
└── invoices/        # Generated invoice PDFs
```

### config.yaml

```yaml
# Default hourly rate (if not set per-client)
default_rate: 150.00

# Invoice settings
invoice:
  your_name: "Your Name"
  your_email: "you@example.com"
  your_address: |
    123 Main Street
    Athens, GA 30601
  tax_rate: 0.0
  payment_terms: "Net 30"
  invoice_prefix: "INV"

# Display preferences
display:
  currency: "USD"
  date_format: "2006-01-02"
  time_format: "3:04 PM"
  theme: "classic"  # classic, minimal, cyberpunk
```

## Data Model

### Clients
- Name, contact info, hourly rate
- Active/archived status
- Notes and metadata

### Time Entries
- Start/end timestamps
- Client reference
- Task description
- Billable flag
- Hourly rate (captured at time of entry)

### Invoices
- Invoice number (auto-generated)
- Client reference
- Date range
- Line items (from time entries)
- Status (draft, sent, paid, overdue)
- Payment details

## Development

### Prerequisites
- Go 1.21 or higher
- SQLite3

### Building from source

```bash
git clone https://github.com/yourusername/timesink.git
cd timesink
go mod download
go build -o timesink ./cmd/timesink
```

### Running tests

```bash
go test ./...
```

### Project structure

```
timesink/
├── cmd/
│   └── timesink/          # Main entry point
│       └── main.go
├── internal/
│   ├── db/                # Database layer (SQLite)
│   ├── models/            # Domain models
│   ├── tui/               # Terminal UI components
│   ├── timer/             # Timer logic
│   ├── invoice/           # Invoice generation
│   └── reports/           # Reports and analytics
├── mockups/               # ASCII UI mockups
├── go.mod
├── go.sum
└── README.md
```

## Roadmap

- [x] ASCII mockups and design
- [ ] Core data models and SQLite schema
- [ ] Basic CLI commands (start, stop, log)
- [ ] Interactive TUI with keyboard navigation
- [ ] Client management
- [ ] Time entry CRUD operations
- [ ] Invoice generation (PDF export)
- [ ] Reports and statistics
- [ ] Configuration file support
- [ ] Data export (CSV, JSON)
- [ ] Timer notifications
- [ ] Multi-currency support
- [ ] Cloud sync (optional)

## Contributing

Contributions welcome! This is a personal project but I'm happy to review PRs.

1. Fork the repo
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Why "timesink"?

Because naming things is hard, and irony is easy. Also because time tracking tools often become the very thing they're supposed to prevent: a huge waste of time. This one aims to be different.

## Author

Built with ☕ by a developer who bills by the hour and got tired of spreadsheets.

---

**Pro tip:** Set up a shell alias for quick timer operations:

```bash
alias ts='timesink'
alias tstart='timesink start'
alias tstop='timesink stop'
```

Now you can just `tstart "Cox Auto" "security audit"` and get back to work.