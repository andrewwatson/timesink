package service

import (
	"context"
	"time"

	"github.com/andy/timesink/internal/domain"
	"github.com/andy/timesink/internal/repository"
)

// WeekSummary provides weekly time tracking analytics
type WeekSummary struct {
	TotalHours    float64
	BillableHours float64
	TotalValue    float64
	ByClient      map[int64]float64 // Hours by client ID
	ByDay         map[time.Weekday]float64
}

// ClientSummary provides client-specific time and revenue analytics
type ClientSummary struct {
	ClientID      int64
	TotalHours    float64
	BillableHours float64
	TotalValue    float64
	UnbilledValue float64
	Entries       []*domain.TimeEntry
}

// DailySummary provides daily time tracking analytics
type DailySummary struct {
	Date          time.Time
	TotalHours    float64
	BillableHours float64
	TotalValue    float64
	Entries       []*domain.TimeEntry
}

// ReportService provides aggregations and analytics
type ReportService interface {
	// Time tracking summaries
	GetWeekSummary(ctx context.Context, weekStart time.Time) (*WeekSummary, error)
	GetClientSummary(ctx context.Context, clientID int64, start, end time.Time) (*ClientSummary, error)
	GetDailySummary(ctx context.Context, date time.Time) (*DailySummary, error)

	// Financial summaries
	GetOutstandingTotal(ctx context.Context) (float64, error) // Unpaid invoices
	GetUnbilledTotal(ctx context.Context) (float64, error)    // Time not yet invoiced
	GetRevenueByMonth(ctx context.Context, year int) (map[time.Month]float64, error)
}

type reportService struct {
	entryRepo   repository.TimeEntryRepository
	invoiceRepo repository.InvoiceRepository
}

// NewReportService creates a new report service
func NewReportService(
	entryRepo repository.TimeEntryRepository,
	invoiceRepo repository.InvoiceRepository,
) ReportService {
	return &reportService{
		entryRepo:   entryRepo,
		invoiceRepo: invoiceRepo,
	}
}

func (s *reportService) GetWeekSummary(ctx context.Context, weekStart time.Time) (*WeekSummary, error) {
	// Ensure weekStart is actually a Monday (start of week)
	for weekStart.Weekday() != time.Monday {
		weekStart = weekStart.AddDate(0, 0, -1)
	}

	// Calculate week end (Sunday)
	weekEnd := weekStart.AddDate(0, 0, 7)

	// Get all entries for the week
	entries, err := s.entryRepo.List(ctx, nil, &weekStart, &weekEnd, true)
	if err != nil {
		return nil, err
	}

	summary := &WeekSummary{
		ByClient: make(map[int64]float64),
		ByDay:    make(map[time.Weekday]float64),
	}

	for _, entry := range entries {
		hours := entry.Duration().Hours()
		value := entry.Amount()

		summary.TotalHours += hours
		if entry.IsBillable {
			summary.BillableHours += hours
		}
		summary.TotalValue += value

		// Aggregate by client
		summary.ByClient[entry.ClientID] += hours

		// Aggregate by day of week
		weekday := entry.StartTime.Weekday()
		summary.ByDay[weekday] += hours
	}

	return summary, nil
}

func (s *reportService) GetClientSummary(
	ctx context.Context,
	clientID int64,
	start, end time.Time,
) (*ClientSummary, error) {
	// Get all entries for client in period
	entries, err := s.entryRepo.List(ctx, &clientID, &start, &end, true)
	if err != nil {
		return nil, err
	}

	summary := &ClientSummary{
		ClientID: clientID,
		Entries:  entries,
	}

	for _, entry := range entries {
		hours := entry.Duration().Hours()
		value := entry.Amount()

		summary.TotalHours += hours
		if entry.IsBillable {
			summary.BillableHours += hours
		}
		summary.TotalValue += value

		// Track unbilled value
		if entry.InvoiceID == nil && entry.IsBillable {
			summary.UnbilledValue += value
		}
	}

	return summary, nil
}

func (s *reportService) GetDailySummary(ctx context.Context, date time.Time) (*DailySummary, error) {
	// Normalize to start of day
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.AddDate(0, 0, 1)

	// Get all entries for the day
	entries, err := s.entryRepo.List(ctx, nil, &startOfDay, &endOfDay, true)
	if err != nil {
		return nil, err
	}

	summary := &DailySummary{
		Date:    date,
		Entries: entries,
	}

	for _, entry := range entries {
		hours := entry.Duration().Hours()
		value := entry.Amount()

		summary.TotalHours += hours
		if entry.IsBillable {
			summary.BillableHours += hours
		}
		summary.TotalValue += value
	}

	return summary, nil
}

func (s *reportService) GetOutstandingTotal(ctx context.Context) (float64, error) {
	// Get invoices with status sent or overdue
	sentStatus := domain.InvoiceStatusSent
	overdueStatus := domain.InvoiceStatusOverdue

	sentInvoices, err := s.invoiceRepo.List(ctx, nil, &sentStatus)
	if err != nil {
		return 0, err
	}

	overdueInvoices, err := s.invoiceRepo.List(ctx, nil, &overdueStatus)
	if err != nil {
		return 0, err
	}

	total := 0.0
	for _, invoice := range sentInvoices {
		total += invoice.Total
	}
	for _, invoice := range overdueInvoices {
		total += invoice.Total
	}

	return total, nil
}

func (s *reportService) GetUnbilledTotal(ctx context.Context) (float64, error) {
	// Get all unbilled entries (no invoice_id)
	entries, err := s.entryRepo.List(ctx, nil, nil, nil, false)
	if err != nil {
		return 0, err
	}

	total := 0.0
	for _, entry := range entries {
		if entry.InvoiceID == nil && entry.IsBillable {
			total += entry.Amount()
		}
	}

	return total, nil
}

func (s *reportService) GetRevenueByMonth(ctx context.Context, year int) (map[time.Month]float64, error) {
	// Get all paid invoices for the year
	paidStatus := domain.InvoiceStatusPaid
	invoices, err := s.invoiceRepo.List(ctx, nil, &paidStatus)
	if err != nil {
		return nil, err
	}

	revenue := make(map[time.Month]float64)

	// Initialize all months to 0
	for m := time.January; m <= time.December; m++ {
		revenue[m] = 0
	}

	for _, invoice := range invoices {
		// Use paid date if available, otherwise use updated date
		var paymentDate time.Time
		if invoice.PaidDate != nil {
			paymentDate = *invoice.PaidDate
		} else {
			paymentDate = invoice.UpdatedAt
		}

		// Only include invoices paid in the requested year
		if paymentDate.Year() == year {
			month := paymentDate.Month()
			revenue[month] += invoice.Total
		}
	}

	return revenue, nil
}
