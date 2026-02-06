package domain

import (
	"errors"
	"time"
)

type InvoiceStatus string

const (
	InvoiceStatusDraft     InvoiceStatus = "draft"
	InvoiceStatusFinalized InvoiceStatus = "finalized"
	InvoiceStatusSent      InvoiceStatus = "sent"
	InvoiceStatusPaid      InvoiceStatus = "paid"
	InvoiceStatusOverdue   InvoiceStatus = "overdue"
)

type Invoice struct {
	ID            int64
	InvoiceNumber string
	ClientID      int64
	PeriodStart   time.Time
	PeriodEnd     time.Time
	Subtotal      float64
	TaxRate       float64
	TaxAmount     float64
	Total         float64
	Status        InvoiceStatus
	DueDate       *time.Time
	PaidDate      *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Related data (populated by repository)
	LineItems []*InvoiceLineItem
	Client    *Client
}

type InvoiceLineItem struct {
	ID          int64
	InvoiceID   int64
	EntryID     int64
	Date        time.Time
	Description string
	Hours       float64
	Rate        float64
	Amount      float64
}

// NewInvoice creates a new draft invoice
func NewInvoice(invoiceNumber string, clientID int64, periodStart, periodEnd time.Time) *Invoice {
	now := time.Now()
	return &Invoice{
		InvoiceNumber: invoiceNumber,
		ClientID:      clientID,
		PeriodStart:   periodStart,
		PeriodEnd:     periodEnd,
		Status:        InvoiceStatusDraft,
		CreatedAt:     now,
		UpdatedAt:     now,
		LineItems:     make([]*InvoiceLineItem, 0),
	}
}

// CanEdit returns true if the invoice can be modified
func (i *Invoice) CanEdit() bool {
	return i.Status == InvoiceStatusDraft
}

// IsFinalized returns true if the invoice is finalized or later
func (i *Invoice) IsFinalized() bool {
	return i.Status != InvoiceStatusDraft
}

// Finalize locks the invoice and prevents further edits
func (i *Invoice) Finalize() {
	if i.Status == InvoiceStatusDraft {
		i.Status = InvoiceStatusFinalized
		i.UpdatedAt = time.Now()
	}
}

// CalculateTotals recalculates subtotal, tax, and total from line items
func (i *Invoice) CalculateTotals() {
	i.Subtotal = 0
	for _, item := range i.LineItems {
		i.Subtotal += item.Amount
	}
	i.TaxAmount = i.Subtotal * i.TaxRate
	i.Total = i.Subtotal + i.TaxAmount
	i.UpdatedAt = time.Now()
}

// Validate returns an error if the invoice is invalid
func (i *Invoice) Validate() error {
	if i.InvoiceNumber == "" {
		return errors.New("invoice number is required")
	}
	if i.ClientID <= 0 {
		return errors.New("client ID is required")
	}
	if i.PeriodStart.IsZero() {
		return errors.New("period start is required")
	}
	if i.PeriodEnd.IsZero() {
		return errors.New("period end is required")
	}
	if i.PeriodEnd.Before(i.PeriodStart) {
		return errors.New("period end must be after period start")
	}
	if i.TaxRate < 0 || i.TaxRate > 1 {
		return errors.New("tax rate must be between 0 and 1")
	}
	return nil
}
