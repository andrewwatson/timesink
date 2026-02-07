package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/andy/timesink/internal/db"
	"github.com/andy/timesink/internal/domain"
)

// InvoiceRepo is a SQLite implementation of InvoiceRepository
type InvoiceRepo struct {
	db *db.DB
}

// NewInvoiceRepo creates a new InvoiceRepo
func NewInvoiceRepo(database *db.DB) *InvoiceRepo {
	return &InvoiceRepo{db: database}
}

// Create inserts a new invoice into the database
func (r *InvoiceRepo) Create(ctx context.Context, invoice *domain.Invoice) error {
	if err := invoice.Validate(); err != nil {
		return fmt.Errorf("invalid invoice: %w", err)
	}

	query := `
		INSERT INTO invoices (
			invoice_number, client_id, period_start, period_end,
			subtotal, tax_rate, tax_amount, total, status,
			due_date, paid_date, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var dueDate, paidDate interface{}
	if invoice.DueDate != nil {
		dueDate = invoice.DueDate.Format(timeLayout)
	}
	if invoice.PaidDate != nil {
		paidDate = invoice.PaidDate.Format(timeLayout)
	}

	result, err := r.db.ExecContext(ctx, query,
		invoice.InvoiceNumber,
		invoice.ClientID,
		invoice.PeriodStart.Format(timeLayout),
		invoice.PeriodEnd.Format(timeLayout),
		invoice.Subtotal,
		invoice.TaxRate,
		invoice.TaxAmount,
		invoice.Total,
		string(invoice.Status),
		dueDate,
		paidDate,
		invoice.CreatedAt.Format(timeLayout),
		invoice.UpdatedAt.Format(timeLayout),
	)
	if err != nil {
		return fmt.Errorf("failed to create invoice: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get invoice ID: %w", err)
	}

	invoice.ID = id
	return nil
}

// GetByID retrieves an invoice by ID
func (r *InvoiceRepo) GetByID(ctx context.Context, id int64) (*domain.Invoice, error) {
	query := `
		SELECT id, invoice_number, client_id, period_start, period_end,
		       subtotal, tax_rate, tax_amount, total, status,
		       due_date, paid_date, created_at, updated_at
		FROM invoices
		WHERE id = ?
	`

	invoice := &domain.Invoice{}
	var periodStart, periodEnd, status string
	var dueDate, paidDate, createdAt, updatedAt sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&invoice.ID,
		&invoice.InvoiceNumber,
		&invoice.ClientID,
		&periodStart,
		&periodEnd,
		&invoice.Subtotal,
		&invoice.TaxRate,
		&invoice.TaxAmount,
		&invoice.Total,
		&status,
		&dueDate,
		&paidDate,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("invoice not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	if err := scanInvoice(invoice, periodStart, periodEnd, status, dueDate, paidDate, createdAt, updatedAt); err != nil {
		return nil, err
	}

	return invoice, nil
}

// GetByNumber retrieves an invoice by invoice number
func (r *InvoiceRepo) GetByNumber(ctx context.Context, number string) (*domain.Invoice, error) {
	query := `
		SELECT id, invoice_number, client_id, period_start, period_end,
		       subtotal, tax_rate, tax_amount, total, status,
		       due_date, paid_date, created_at, updated_at
		FROM invoices
		WHERE invoice_number = ?
	`

	invoice := &domain.Invoice{}
	var periodStart, periodEnd, status string
	var dueDate, paidDate, createdAt, updatedAt sql.NullString

	err := r.db.QueryRowContext(ctx, query, number).Scan(
		&invoice.ID,
		&invoice.InvoiceNumber,
		&invoice.ClientID,
		&periodStart,
		&periodEnd,
		&invoice.Subtotal,
		&invoice.TaxRate,
		&invoice.TaxAmount,
		&invoice.Total,
		&status,
		&dueDate,
		&paidDate,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("invoice not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	if err := scanInvoice(invoice, periodStart, periodEnd, status, dueDate, paidDate, createdAt, updatedAt); err != nil {
		return nil, err
	}

	return invoice, nil
}

// List retrieves invoices with optional filters
func (r *InvoiceRepo) List(ctx context.Context, clientID *int64, status *domain.InvoiceStatus) ([]*domain.Invoice, error) {
	query := `
		SELECT id, invoice_number, client_id, period_start, period_end,
		       subtotal, tax_rate, tax_amount, total, status,
		       due_date, paid_date, created_at, updated_at
		FROM invoices
		WHERE 1=1
	`
	args := make([]interface{}, 0)

	if clientID != nil {
		query += " AND client_id = ?"
		args = append(args, *clientID)
	}

	if status != nil {
		query += " AND status = ?"
		args = append(args, string(*status))
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list invoices: %w", err)
	}
	defer rows.Close()

	invoices := make([]*domain.Invoice, 0)
	for rows.Next() {
		invoice := &domain.Invoice{}
		var periodStart, periodEnd, statusStr string
		var dueDate, paidDate, createdAt, updatedAt sql.NullString

		err := rows.Scan(
			&invoice.ID,
			&invoice.InvoiceNumber,
			&invoice.ClientID,
			&periodStart,
			&periodEnd,
			&invoice.Subtotal,
			&invoice.TaxRate,
			&invoice.TaxAmount,
			&invoice.Total,
			&statusStr,
			&dueDate,
			&paidDate,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan invoice: %w", err)
		}

		if err := scanInvoice(invoice, periodStart, periodEnd, statusStr, dueDate, paidDate, createdAt, updatedAt); err != nil {
			return nil, err
		}

		invoices = append(invoices, invoice)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating invoices: %w", err)
	}

	return invoices, nil
}

// Update updates an existing invoice
func (r *InvoiceRepo) Update(ctx context.Context, invoice *domain.Invoice) error {
	if err := invoice.Validate(); err != nil {
		return fmt.Errorf("invalid invoice: %w", err)
	}

	query := `
		UPDATE invoices
		SET invoice_number = ?, client_id = ?, period_start = ?, period_end = ?,
		    subtotal = ?, tax_rate = ?, tax_amount = ?, total = ?, status = ?,
		    due_date = ?, paid_date = ?, updated_at = ?
		WHERE id = ?
	`

	var dueDate, paidDate interface{}
	if invoice.DueDate != nil {
		dueDate = invoice.DueDate.Format(timeLayout)
	}
	if invoice.PaidDate != nil {
		paidDate = invoice.PaidDate.Format(timeLayout)
	}

	invoice.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		invoice.InvoiceNumber,
		invoice.ClientID,
		invoice.PeriodStart.Format(timeLayout),
		invoice.PeriodEnd.Format(timeLayout),
		invoice.Subtotal,
		invoice.TaxRate,
		invoice.TaxAmount,
		invoice.Total,
		string(invoice.Status),
		dueDate,
		paidDate,
		invoice.UpdatedAt.Format(timeLayout),
		invoice.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update invoice: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("invoice not found")
	}

	return nil
}

// AddLineItem adds a line item to an invoice
func (r *InvoiceRepo) AddLineItem(ctx context.Context, invoiceID int64, item *domain.InvoiceLineItem) error {
	query := `
		INSERT INTO invoice_line_items (invoice_id, entry_id, date, description, hours, rate, amount)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query,
		invoiceID,
		item.EntryID,
		item.Date.Format(timeLayout),
		item.Description,
		item.Hours,
		item.Rate,
		item.Amount,
	)
	if err != nil {
		return fmt.Errorf("failed to add line item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get line item ID: %w", err)
	}

	item.ID = id
	item.InvoiceID = invoiceID
	return nil
}

// DeleteLineItem removes a specific line item from an invoice
func (r *InvoiceRepo) DeleteLineItem(ctx context.Context, invoiceID int64, lineItemID int64) error {
	query := `
		DELETE FROM invoice_line_items
		WHERE id = ? AND invoice_id = ?
	`

	result, err := r.db.ExecContext(ctx, query, lineItemID, invoiceID)
	if err != nil {
		return fmt.Errorf("failed to delete line item: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("line item not found")
	}

	return nil
}

// GetLineItems retrieves all line items for an invoice
func (r *InvoiceRepo) GetLineItems(ctx context.Context, invoiceID int64) ([]*domain.InvoiceLineItem, error) {
	query := `
		SELECT id, invoice_id, entry_id, date, description, hours, rate, amount
		FROM invoice_line_items
		WHERE invoice_id = ?
		ORDER BY date
	`

	rows, err := r.db.QueryContext(ctx, query, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get line items: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.InvoiceLineItem, 0)
	for rows.Next() {
		item := &domain.InvoiceLineItem{}
		var date string

		err := rows.Scan(
			&item.ID,
			&item.InvoiceID,
			&item.EntryID,
			&date,
			&item.Description,
			&item.Hours,
			&item.Rate,
			&item.Amount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan line item: %w", err)
		}

		if item.Date, err = parseTime(date); err != nil {
			return nil, fmt.Errorf("failed to parse date: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating line items: %w", err)
	}

	return items, nil
}

// GetNextInvoiceNumber generates the next invoice number in format "PREFIX-YEAR-SEQUENCE"
func (r *InvoiceRepo) GetNextInvoiceNumber(ctx context.Context, prefix string, year int) (string, error) {
	// Find the highest sequence number for the given prefix and year
	query := `
		SELECT invoice_number
		FROM invoices
		WHERE invoice_number LIKE ?
		ORDER BY invoice_number DESC
		LIMIT 1
	`

	pattern := fmt.Sprintf("%s-%d-%%", prefix, year)
	var lastNumber string

	err := r.db.QueryRowContext(ctx, query, pattern).Scan(&lastNumber)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No existing invoices for this year, start at 001
			return fmt.Sprintf("%s-%d-001", prefix, year), nil
		}
		return "", fmt.Errorf("failed to get last invoice number: %w", err)
	}

	// Parse the sequence number from the last invoice
	// Format: PREFIX-YEAR-SEQUENCE (e.g., "INV-2026-005")
	var lastSeq int
	_, err = fmt.Sscanf(lastNumber, prefix+"-%d-%d", &year, &lastSeq)
	if err != nil {
		// Fallback: start at 001 if we can't parse
		return fmt.Sprintf("%s-%d-001", prefix, year), nil
	}

	// Increment and format
	nextSeq := lastSeq + 1
	return fmt.Sprintf("%s-%d-%03d", prefix, year, nextSeq), nil
}

// scanInvoice is a helper to parse invoice fields
func scanInvoice(invoice *domain.Invoice, periodStart, periodEnd, status string, dueDate, paidDate, createdAt, updatedAt sql.NullString) error {
	var err error

	if invoice.PeriodStart, err = parseTime(periodStart); err != nil {
		return fmt.Errorf("failed to parse period_start: %w", err)
	}

	if invoice.PeriodEnd, err = parseTime(periodEnd); err != nil {
		return fmt.Errorf("failed to parse period_end: %w", err)
	}

	invoice.Status = domain.InvoiceStatus(status)

	if dueDate.Valid {
		t, err := parseTime(dueDate.String)
		if err != nil {
			return fmt.Errorf("failed to parse due_date: %w", err)
		}
		invoice.DueDate = &t
	}

	if paidDate.Valid {
		t, err := parseTime(paidDate.String)
		if err != nil {
			return fmt.Errorf("failed to parse paid_date: %w", err)
		}
		invoice.PaidDate = &t
	}

	if invoice.CreatedAt, err = parseTime(createdAt.String); err != nil {
		return fmt.Errorf("failed to parse created_at: %w", err)
	}

	if invoice.UpdatedAt, err = parseTime(updatedAt.String); err != nil {
		return fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return nil
}
