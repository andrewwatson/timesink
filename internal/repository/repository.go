package repository

import (
	"context"
	"time"

	"github.com/andy/timesink/internal/domain"
)

// ClientRepository manages client persistence
type ClientRepository interface {
	Create(ctx context.Context, client *domain.Client) error
	GetByID(ctx context.Context, id int64) (*domain.Client, error)
	GetByName(ctx context.Context, name string) (*domain.Client, error)
	List(ctx context.Context, includeArchived bool) ([]*domain.Client, error)
	Update(ctx context.Context, client *domain.Client) error
	Archive(ctx context.Context, id int64) error
	Unarchive(ctx context.Context, id int64) error
}

// TimeEntryRepository manages time entry persistence with audit trail
type TimeEntryRepository interface {
	Create(ctx context.Context, entry *domain.TimeEntry) error
	GetByID(ctx context.Context, id int64) (*domain.TimeEntry, error)
	Update(ctx context.Context, entry *domain.TimeEntry, reason string) error // Creates audit record
	SoftDelete(ctx context.Context, id int64, reason string) error
	List(ctx context.Context, clientID *int64, start, end *time.Time, includeLocked bool) ([]*domain.TimeEntry, error)
	GetUnbilledByClient(ctx context.Context, clientID int64, start, end time.Time) ([]*domain.TimeEntry, error)
	IsLocked(ctx context.Context, id int64) (bool, error)
	LockForInvoice(ctx context.Context, entryIDs []int64, invoiceID int64) error
	GetHistory(ctx context.Context, entryID int64) ([]*domain.EntryHistory, error)
}

// InvoiceRepository manages invoice persistence
type InvoiceRepository interface {
	Create(ctx context.Context, invoice *domain.Invoice) error
	GetByID(ctx context.Context, id int64) (*domain.Invoice, error)
	GetByNumber(ctx context.Context, number string) (*domain.Invoice, error)
	List(ctx context.Context, clientID *int64, status *domain.InvoiceStatus) ([]*domain.Invoice, error)
	Update(ctx context.Context, invoice *domain.Invoice) error
	AddLineItem(ctx context.Context, invoiceID int64, item *domain.InvoiceLineItem) error
	// DeleteLineItem removes a specific line item from an invoice
	DeleteLineItem(ctx context.Context, invoiceID int64, lineItemID int64) error
	GetLineItems(ctx context.Context, invoiceID int64) ([]*domain.InvoiceLineItem, error)
	GetNextInvoiceNumber(ctx context.Context, prefix string, year int) (string, error)
}

// TimerRepository manages the active timer state (singleton)
type TimerRepository interface {
	Get(ctx context.Context) (*domain.ActiveTimer, error) // Returns nil if no active timer
	Save(ctx context.Context, timer *domain.ActiveTimer) error
	Delete(ctx context.Context) error
}
