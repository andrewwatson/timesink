package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andy/timesink/internal/domain"
)

// mock implementations
type mockInvoiceRepo struct {
	invoices  map[int64]*domain.Invoice
	lineItems map[int64][]*domain.InvoiceLineItem
	updated   *domain.Invoice
}

func (m *mockInvoiceRepo) Create(ctx context.Context, invoice *domain.Invoice) error { return nil }
func (m *mockInvoiceRepo) GetByID(ctx context.Context, id int64) (*domain.Invoice, error) {
	if inv, ok := m.invoices[id]; ok {
		return inv, nil
	}
	return nil, nil
}
func (m *mockInvoiceRepo) GetByNumber(ctx context.Context, number string) (*domain.Invoice, error) {
	return nil, nil
}
func (m *mockInvoiceRepo) List(ctx context.Context, clientID *int64, status *domain.InvoiceStatus) ([]*domain.Invoice, error) {
	return nil, nil
}
func (m *mockInvoiceRepo) Update(ctx context.Context, invoice *domain.Invoice) error {
	m.updated = invoice
	return nil
}
func (m *mockInvoiceRepo) AddLineItem(ctx context.Context, invoiceID int64, item *domain.InvoiceLineItem) error {
	m.lineItems[invoiceID] = append(m.lineItems[invoiceID], item)
	return nil
}
func (m *mockInvoiceRepo) GetLineItems(ctx context.Context, invoiceID int64) ([]*domain.InvoiceLineItem, error) {
	items := m.lineItems[invoiceID]
	// return a copy
	out := make([]*domain.InvoiceLineItem, len(items))
	copy(out, items)
	return out, nil
}
func (m *mockInvoiceRepo) GetNextInvoiceNumber(ctx context.Context, prefix string, year int) (string, error) {
	return "INV-2026-001", nil
}
func (m *mockInvoiceRepo) DeleteLineItem(ctx context.Context, invoiceID int64, lineItemID int64) error {
	items := m.lineItems[invoiceID]
	for i, it := range items {
		if it.ID == lineItemID {
			// remove
			m.lineItems[invoiceID] = append(items[:i], items[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

type mockEntryRepo struct{}

func (m *mockEntryRepo) Create(ctx context.Context, entry *domain.TimeEntry) error { return nil }
func (m *mockEntryRepo) GetByID(ctx context.Context, id int64) (*domain.TimeEntry, error) {
	return nil, nil
}
func (m *mockEntryRepo) Update(ctx context.Context, entry *domain.TimeEntry, reason string) error {
	return nil
}
func (m *mockEntryRepo) SoftDelete(ctx context.Context, id int64, reason string) error { return nil }
func (m *mockEntryRepo) List(ctx context.Context, clientID *int64, start, end *time.Time, includeLocked bool) ([]*domain.TimeEntry, error) {
	return nil, nil
}
func (m *mockEntryRepo) GetUnbilledByClient(ctx context.Context, clientID int64, start, end time.Time) ([]*domain.TimeEntry, error) {
	return nil, nil
}
func (m *mockEntryRepo) IsLocked(ctx context.Context, id int64) (bool, error) { return false, nil }
func (m *mockEntryRepo) LockForInvoice(ctx context.Context, entryIDs []int64, invoiceID int64) error {
	return nil
}
func (m *mockEntryRepo) GetHistory(ctx context.Context, entryID int64) ([]*domain.EntryHistory, error) {
	return nil, nil
}

type mockClientRepo struct{}

func (m *mockClientRepo) Create(ctx context.Context, client *domain.Client) error { return nil }
func (m *mockClientRepo) GetByID(ctx context.Context, id int64) (*domain.Client, error) {
	return &domain.Client{ID: id, Name: "ACME"}, nil
}
func (m *mockClientRepo) GetByName(ctx context.Context, name string) (*domain.Client, error) {
	return nil, nil
}
func (m *mockClientRepo) List(ctx context.Context, includeArchived bool) ([]*domain.Client, error) {
	return nil, nil
}
func (m *mockClientRepo) Update(ctx context.Context, client *domain.Client) error { return nil }
func (m *mockClientRepo) Archive(ctx context.Context, id int64) error             { return nil }
func (m *mockClientRepo) Unarchive(ctx context.Context, id int64) error           { return nil }

func TestRemoveEntryFromInvoice_Success(t *testing.T) {
	ctx := context.Background()

	// Setup invoice with two line items
	inv := domain.NewInvoice("INV-2026-001", 1, time.Now().Add(-24*time.Hour), time.Now())
	inv.ID = 10
	inv.TaxRate = 0.10

	li1 := &domain.InvoiceLineItem{ID: 1, InvoiceID: inv.ID, EntryID: 100, Hours: 2, Rate: 50, Amount: 100}
	li2 := &domain.InvoiceLineItem{ID: 2, InvoiceID: inv.ID, EntryID: 101, Hours: 1, Rate: 75, Amount: 75}

	mockInv := &mockInvoiceRepo{
		invoices:  map[int64]*domain.Invoice{inv.ID: inv},
		lineItems: map[int64][]*domain.InvoiceLineItem{inv.ID: []*domain.InvoiceLineItem{li1, li2}},
	}

	svc := &invoiceService{
		invoiceRepo: mockInv,
		entryRepo:   &mockEntryRepo{},
		clientRepo:  &mockClientRepo{},
	}

	// Remove entry 100 (li1)
	if err := svc.RemoveEntryFromInvoice(ctx, inv.ID, 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ensure line item removed
	items, _ := mockInv.GetLineItems(ctx, inv.ID)
	if len(items) != 1 {
		t.Fatalf("expected 1 line item after deletion, got %d", len(items))
	}

	// Ensure totals recalculated and Update was called
	if mockInv.updated == nil {
		t.Fatalf("expected invoice update to be called")
	}

	// Updated invoice should have subtotal equal to remaining amount (75)
	if mockInv.updated.Subtotal != 75 {
		t.Fatalf("expected subtotal 75, got %v", mockInv.updated.Subtotal)
	}
}

func TestRemoveEntryFromInvoice_NotFound(t *testing.T) {
	ctx := context.Background()

	inv := domain.NewInvoice("INV-2026-001", 1, time.Now().Add(-24*time.Hour), time.Now())
	inv.ID = 11

	mockInv := &mockInvoiceRepo{
		invoices:  map[int64]*domain.Invoice{inv.ID: inv},
		lineItems: map[int64][]*domain.InvoiceLineItem{inv.ID: []*domain.InvoiceLineItem{}},
	}

	svc := &invoiceService{
		invoiceRepo: mockInv,
		entryRepo:   &mockEntryRepo{},
		clientRepo:  &mockClientRepo{},
	}

	err := svc.RemoveEntryFromInvoice(ctx, inv.ID, 999)
	if err == nil {
		t.Fatalf("expected error for missing entry")
	}
}
