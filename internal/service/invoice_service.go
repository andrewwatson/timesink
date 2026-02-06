package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andy/timesink/internal/domain"
	"github.com/andy/timesink/internal/repository"
)

var (
	ErrInvoiceNotEditable = errors.New("invoice cannot be edited after finalization")
	ErrEntryAlreadyLocked = errors.New("entry is already locked to an invoice")
	ErrEntryNotFound      = errors.New("time entry not found")
)

// InvoiceService manages invoice lifecycle and entry locking
type InvoiceService interface {
	// CreateDraft creates a new draft invoice with auto-generated number
	CreateDraft(ctx context.Context, clientID int64, periodStart, periodEnd time.Time, prefix string) (*domain.Invoice, error)

	// AddEntriesToInvoice adds time entries to a draft invoice
	AddEntriesToInvoice(ctx context.Context, invoiceID int64, entryIDs []int64) error

	// RemoveEntryFromInvoice removes an entry from a draft invoice
	RemoveEntryFromInvoice(ctx context.Context, invoiceID int64, entryID int64) error

	// CalculateTotals recalculates invoice totals with tax
	CalculateTotals(ctx context.Context, invoiceID int64, taxRate float64) error

	// Finalize locks the invoice and all associated entries
	Finalize(ctx context.Context, invoiceID int64) error

	// MarkSent updates invoice status to sent
	MarkSent(ctx context.Context, invoiceID int64) error

	// MarkPaid updates invoice status to paid with payment date
	MarkPaid(ctx context.Context, invoiceID int64, paidDate time.Time) error

	// CheckOverdue updates overdue status for sent invoices past due date
	CheckOverdue(ctx context.Context) error

	// GetInvoice retrieves an invoice by ID
	GetInvoice(ctx context.Context, id int64) (*domain.Invoice, error)

	// ListInvoices lists invoices with optional filters
	ListInvoices(ctx context.Context, clientID *int64, status *domain.InvoiceStatus) ([]*domain.Invoice, error)
}

type invoiceService struct {
	invoiceRepo repository.InvoiceRepository
	entryRepo   repository.TimeEntryRepository
	clientRepo  repository.ClientRepository
}

// NewInvoiceService creates a new invoice service
func NewInvoiceService(
	invoiceRepo repository.InvoiceRepository,
	entryRepo repository.TimeEntryRepository,
	clientRepo repository.ClientRepository,
) InvoiceService {
	return &invoiceService{
		invoiceRepo: invoiceRepo,
		entryRepo:   entryRepo,
		clientRepo:  clientRepo,
	}
}

func (s *invoiceService) CreateDraft(
	ctx context.Context,
	clientID int64,
	periodStart, periodEnd time.Time,
	prefix string,
) (*domain.Invoice, error) {
	// Verify client exists
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, errors.New("client not found")
	}

	// Generate invoice number
	year := periodEnd.Year()
	invoiceNumber, err := s.invoiceRepo.GetNextInvoiceNumber(ctx, prefix, year)
	if err != nil {
		return nil, fmt.Errorf("failed to generate invoice number: %w", err)
	}

	// Create invoice
	invoice := domain.NewInvoice(invoiceNumber, clientID, periodStart, periodEnd)
	if err := invoice.Validate(); err != nil {
		return nil, err
	}

	if err := s.invoiceRepo.Create(ctx, invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}

func (s *invoiceService) AddEntriesToInvoice(ctx context.Context, invoiceID int64, entryIDs []int64) error {
	// Get invoice
	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return err
	}
	if invoice == nil {
		return errors.New("invoice not found")
	}

	// Check invoice is editable
	if !invoice.CanEdit() {
		return ErrInvoiceNotEditable
	}

	// Verify all entries are unlocked
	for _, entryID := range entryIDs {
		locked, err := s.entryRepo.IsLocked(ctx, entryID)
		if err != nil {
			return err
		}
		if locked {
			return fmt.Errorf("%w: entry %d", ErrEntryAlreadyLocked, entryID)
		}

		// Verify entry exists and belongs to invoice client
		entry, err := s.entryRepo.GetByID(ctx, entryID)
		if err != nil {
			return err
		}
		if entry == nil {
			return fmt.Errorf("%w: entry %d", ErrEntryNotFound, entryID)
		}
		if entry.ClientID != invoice.ClientID {
			return fmt.Errorf("entry %d does not belong to invoice client", entryID)
		}
	}

	// Create line items for each entry
	for _, entryID := range entryIDs {
		entry, err := s.entryRepo.GetByID(ctx, entryID)
		if err != nil {
			return err
		}

		lineItem := &domain.InvoiceLineItem{
			InvoiceID:   invoiceID,
			EntryID:     entryID,
			Date:        entry.StartTime,
			Description: entry.Description,
			Hours:       entry.Duration().Hours(),
			Rate:        entry.HourlyRate,
			Amount:      entry.Amount(),
		}

		if err := s.invoiceRepo.AddLineItem(ctx, invoiceID, lineItem); err != nil {
			return err
		}
	}

	return nil
}

func (s *invoiceService) RemoveEntryFromInvoice(ctx context.Context, invoiceID int64, entryID int64) error {
	// Get invoice
	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return err
	}
	if invoice == nil {
		return errors.New("invoice not found")
	}

	// Check invoice is editable
	if !invoice.CanEdit() {
		return ErrInvoiceNotEditable
	}

	// Load line items for invoice and find matching entry
	lineItems, err := s.invoiceRepo.GetLineItems(ctx, invoiceID)
	if err != nil {
		return err
	}

	var target *domain.InvoiceLineItem
	for _, li := range lineItems {
		if li.EntryID == entryID {
			target = li
			break
		}
	}

	if target == nil {
		return errors.New("line item for entry not found on invoice")
	}

	// Delete the line item via repository
	if err := s.invoiceRepo.DeleteLineItem(ctx, invoiceID, target.ID); err != nil {
		return err
	}

	// Recalculate totals using the invoice's tax rate
	return s.CalculateTotals(ctx, invoiceID, invoice.TaxRate)
}

func (s *invoiceService) CalculateTotals(ctx context.Context, invoiceID int64, taxRate float64) error {
	// Get invoice with line items
	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return err
	}
	if invoice == nil {
		return errors.New("invoice not found")
	}

	// Load line items
	lineItems, err := s.invoiceRepo.GetLineItems(ctx, invoiceID)
	if err != nil {
		return err
	}
	invoice.LineItems = lineItems

	// Set tax rate and calculate
	invoice.TaxRate = taxRate
	invoice.CalculateTotals()

	// Save updated invoice
	return s.invoiceRepo.Update(ctx, invoice)
}

func (s *invoiceService) Finalize(ctx context.Context, invoiceID int64) error {
	// Get invoice with line items
	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return err
	}
	if invoice == nil {
		return errors.New("invoice not found")
	}

	// Check invoice is editable
	if !invoice.CanEdit() {
		return ErrInvoiceNotEditable
	}

	// Get line items to lock entries
	lineItems, err := s.invoiceRepo.GetLineItems(ctx, invoiceID)
	if err != nil {
		return err
	}

	if len(lineItems) == 0 {
		return errors.New("cannot finalize invoice with no line items")
	}

	// Extract entry IDs
	entryIDs := make([]int64, len(lineItems))
	for i, item := range lineItems {
		entryIDs[i] = item.EntryID
	}

	// Lock all entries to this invoice
	if err := s.entryRepo.LockForInvoice(ctx, entryIDs, invoiceID); err != nil {
		return fmt.Errorf("failed to lock entries: %w", err)
	}

	// Update invoice status
	invoice.Finalize()
	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return err
	}

	return nil
}

func (s *invoiceService) MarkSent(ctx context.Context, invoiceID int64) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return err
	}
	if invoice == nil {
		return errors.New("invoice not found")
	}

	if invoice.Status == domain.InvoiceStatusDraft {
		return errors.New("cannot mark draft invoice as sent - finalize first")
	}

	invoice.Status = domain.InvoiceStatusSent
	invoice.UpdatedAt = time.Now()

	return s.invoiceRepo.Update(ctx, invoice)
}

func (s *invoiceService) MarkPaid(ctx context.Context, invoiceID int64, paidDate time.Time) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return err
	}
	if invoice == nil {
		return errors.New("invoice not found")
	}

	invoice.Status = domain.InvoiceStatusPaid
	invoice.PaidDate = &paidDate
	invoice.UpdatedAt = time.Now()

	return s.invoiceRepo.Update(ctx, invoice)
}

func (s *invoiceService) CheckOverdue(ctx context.Context) error {
	// Get all sent invoices
	sentStatus := domain.InvoiceStatusSent
	invoices, err := s.invoiceRepo.List(ctx, nil, &sentStatus)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, invoice := range invoices {
		if invoice.DueDate != nil && now.After(*invoice.DueDate) {
			invoice.Status = domain.InvoiceStatusOverdue
			invoice.UpdatedAt = now
			if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *invoiceService) GetInvoice(ctx context.Context, id int64) (*domain.Invoice, error) {
	return s.invoiceRepo.GetByID(ctx, id)
}

func (s *invoiceService) ListInvoices(
	ctx context.Context,
	clientID *int64,
	status *domain.InvoiceStatus,
) ([]*domain.Invoice, error) {
	return s.invoiceRepo.List(ctx, clientID, status)
}
