# timesink

A keyboard-driven CLI time tracker for freelancers who bill by the hour.

`timesink` is a terminal-based time tracking tool with an interactive TUI for managing clients, tracking time, generating invoices, and reviewing reports. All data is stored locally in an encrypted SQLite database.

## Installation

### Prerequisites
- Go 1.24 or higher
- A C compiler (for SQLite/sqlcipher)

### From source

```bash
git clone https://github.com/andrewwatson/timesink.git
cd timesink
go build -o timesink ./cmd/timesink
```

## Quick Start

```bash
# Launch the interactive TUI (default)
./timesink

# Or use CLI commands directly
./timesink timer start "Acme Corp" "API integration"
./timesink timer stop
./timesink entries list
```

On first run, you'll be prompted to set a password for database encryption. This password is stored in your system keyring.

## Interactive TUI

Run `timesink` with no arguments to launch the full-screen terminal interface.

### Navigation

| Key | Screen |
|-----|--------|
| `T` | Timer - start, stop, pause timers |
| `E` | Entries - view and create time entries |
| `C` | Clients - manage clients and rates |
| `I` | Invoices - generate and view invoices |
| `R` | Reports - weekly/monthly summaries |
| `S` | Settings - configure invoice defaults |
| `Q` | Quit |

### Common Actions

- `j`/`k` or arrow keys to navigate lists
- `n` to create new items (entries, clients, invoices)
- `Enter` to select or view details
- `Esc` to go back
- `Tab`/`Shift+Tab` to move between form fields
- `Ctrl+S` to save forms

### Timer

Start a timer for a client, then stop it to save a time entry. The timer persists if you quit and relaunch. You cannot quit while a timer is running — stop or discard it first.

### Invoices

Press `n` on the invoices screen to generate an invoice:
1. Select a client with unbilled time
2. Preview the entries and totals
3. Choose where to save the .txt file
4. The invoice is finalized and entries are locked

### Manual Entries

Press `n` on the entries screen to add a time entry manually:
1. Pick a client (or auto-selected if you only have one)
2. Fill in date, start/end times, description, and rate
3. The rate is pre-filled from the client's hourly rate

## CLI Commands

### Timer

```bash
timesink timer start <client> [description]
timesink timer stop
timesink timer pause
timesink timer resume
timesink timer discard
timesink timer status
```

### Clients

```bash
timesink clients list [--archived]
timesink clients add <name> --rate <rate> [--email <email>] [--notes <notes>]
timesink clients edit <id> [--name <name>] [--rate <rate>]
timesink clients archive <id>
timesink clients unarchive <id>
```

### Entries

```bash
timesink entries list [--client <id>] [--start <date>] [--end <date>]
timesink entries add <client> <start_time> <end_time> <description> [--rate <rate>]
timesink entries edit <id> --description <desc> --reason <reason>
timesink entries delete <id> --reason <reason>
timesink entries history <id>
```

### Invoices

```bash
timesink invoices list [--client <id>] [--status <status>]
timesink invoices create <client> [--start <date>] [--end <date>]
timesink invoices add-entries <invoice_id> <entry_ids...> [--tax <rate>]
timesink invoices remove-entry <invoice_id> <entry_id>
timesink invoices finalize <id>
timesink invoices mark-sent <id>
timesink invoices mark-paid <id> [--date <date>]
timesink invoices show <id>
```

### Reset Data

```bash
timesink reset entries     # Delete all entries, invoices, and timer state
timesink reset invoices    # Delete all invoices and unlock time entries
timesink reset all         # Delete everything including clients
```

All reset commands prompt for confirmation before executing.

## Configuration

Data is stored in `~/.config/timesink/`:

```
~/.config/timesink/
├── config.yaml       # User preferences
└── timesink.db       # Encrypted SQLite database
```

### config.yaml

Editable via the Settings screen (`S`) in the TUI, or by editing the file directly:

```yaml
database:
  path: ~/.config/timesink/timesink.db

invoice:
  default_due_days: 30
  default_tax_rate: 0.0
  output_dir: "."
  number_prefix: "INV"

user:
  name: ""
  email: ""
  address: ""
  phone: ""
```

| Setting | Description |
|---------|-------------|
| `invoice.output_dir` | Directory for exported invoice .txt files (default: current directory) |
| `invoice.number_prefix` | Prefix for invoice numbers, e.g. `INV` produces `INV-2026-001` |
| `invoice.default_due_days` | Days until invoice is due (default: 30) |
| `invoice.default_tax_rate` | Tax rate as decimal, e.g. `0.0825` for 8.25% (default: 0) |
| `user.*` | Your info shown on generated invoices |

## Security

- The database is encrypted with [SQLCipher](https://www.zetetic.net/sqlcipher/)
- Your encryption password is stored in the system keyring (macOS Keychain, etc.)
- No data leaves your machine

## License

MIT
