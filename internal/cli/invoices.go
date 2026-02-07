package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/andy/timesink/internal/domain"
	"github.com/spf13/cobra"
)

var invoicesCmd = &cobra.Command{
	Use:   "invoices",
	Short: "Manage invoices",
	Long:  `Create, list, and manage invoices.`,
}

var invoicesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List invoices",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Parse filters
		var clientID *int64
		if cmd.Flags().Changed("client") {
			id, _ := cmd.Flags().GetInt64("client")
			clientID = &id
		}

		var status *domain.InvoiceStatus
		if cmd.Flags().Changed("status") {
			statusStr, _ := cmd.Flags().GetString("status")
			s := domain.InvoiceStatus(statusStr)
			status = &s
		}

		invoices, err := appInstance.InvoiceService.ListInvoices(ctx, clientID, status)
		if err != nil {
			return fmt.Errorf("failed to list invoices: %w", err)
		}

		if len(invoices) == 0 {
			fmt.Println("No invoices found")
			return nil
		}

		// Print table header
		fmt.Printf("%-5s %-15s %-20s %-20s %-12s %-12s\n", "ID", "Number", "Client", "Period", "Total", "Status")
		fmt.Println("--------------------------------------------------------------------------------------------")

		// Print invoices
		for _, invoice := range invoices {
			client, _ := appInstance.ClientRepo.GetByID(ctx, invoice.ClientID)
			clientName := fmt.Sprintf("Client #%d", invoice.ClientID)
			if client != nil {
				clientName = client.Name
			}

			period := fmt.Sprintf("%s - %s",
				invoice.PeriodStart.Format("2006-01-02"),
				invoice.PeriodEnd.Format("2006-01-02"),
			)

			fmt.Printf("%-5d %-15s %-20s %-20s $%-11.2f %-12s\n",
				invoice.ID,
				invoice.InvoiceNumber,
				truncate(clientName, 20),
				truncate(period, 20),
				invoice.Total,
				invoice.Status,
			)
		}

		fmt.Printf("\nTotal: %d invoice(s)\n", len(invoices))
		return nil
	},
}

var invoicesCreateCmd = &cobra.Command{
	Use:   "create [client_id_or_name]",
	Short: "Create a new draft invoice",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Resolve client
		clientID, err := resolveClientID(ctx, args[0])
		if err != nil {
			return fmt.Errorf("failed to resolve client: %w", err)
		}

		// Parse dates
		startStr, _ := cmd.Flags().GetString("start")
		endStr, _ := cmd.Flags().GetString("end")

		start, err := parseDate(startStr)
		if err != nil {
			return fmt.Errorf("invalid start date: %w", err)
		}

		end, err := parseDate(endStr)
		if err != nil {
			return fmt.Errorf("invalid end date: %w", err)
		}

		// Get prefix
		prefix, _ := cmd.Flags().GetString("prefix")
		if prefix == "" {
			prefix = "INV"
		}

		// Create invoice
		invoice, err := appInstance.InvoiceService.CreateDraft(ctx, clientID, start, end, prefix)
		if err != nil {
			return fmt.Errorf("failed to create invoice: %w", err)
		}

		client, _ := appInstance.ClientRepo.GetByID(ctx, clientID)
		clientName := fmt.Sprintf("Client #%d", clientID)
		if client != nil {
			clientName = client.Name
		}

		fmt.Printf("✓ Draft invoice created: %s\n", invoice.InvoiceNumber)
		fmt.Printf("  Client: %s\n", clientName)
		fmt.Printf("  Period: %s to %s\n",
			invoice.PeriodStart.Format("2006-01-02"),
			invoice.PeriodEnd.Format("2006-01-02"),
		)

		return nil
	},
}

var invoicesAddEntriesCmd = &cobra.Command{
	Use:   "add-entries [invoice_id] [entry_ids...]",
	Short: "Add time entries to a draft invoice",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		invoiceID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid invoice ID: %w", err)
		}

		// Parse entry IDs
		entryIDs := make([]int64, 0, len(args)-1)
		for _, idStr := range args[1:] {
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid entry ID '%s': %w", idStr, err)
			}
			entryIDs = append(entryIDs, id)
		}

		// Add entries to invoice
		if err := appInstance.InvoiceService.AddEntriesToInvoice(ctx, invoiceID, entryIDs); err != nil {
			return fmt.Errorf("failed to add entries: %w", err)
		}

		// Recalculate totals
		taxRate, _ := cmd.Flags().GetFloat64("tax")
		if err := appInstance.InvoiceService.CalculateTotals(ctx, invoiceID, taxRate); err != nil {
			return fmt.Errorf("failed to calculate totals: %w", err)
		}

		fmt.Printf("✓ Added %d entries to invoice #%d\n", len(entryIDs), invoiceID)

		// Show updated invoice
		invoice, _ := appInstance.InvoiceService.GetInvoice(ctx, invoiceID)
		if invoice != nil {
			fmt.Printf("  Subtotal: $%.2f\n", invoice.Subtotal)
			fmt.Printf("  Tax: $%.2f\n", invoice.TaxAmount)
			fmt.Printf("  Total: $%.2f\n", invoice.Total)
		}

		return nil
	},
}

var invoicesFinalizeCmd = &cobra.Command{
	Use:   "finalize [id]",
	Short: "Finalize a draft invoice (locks entries)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid invoice ID: %w", err)
		}

		if err := appInstance.InvoiceService.Finalize(ctx, id); err != nil {
			return fmt.Errorf("failed to finalize invoice: %w", err)
		}

		invoice, _ := appInstance.InvoiceService.GetInvoice(ctx, id)
		if invoice != nil {
			fmt.Printf("✓ Invoice finalized: %s\n", invoice.InvoiceNumber)
			fmt.Printf("  Total: $%.2f\n", invoice.Total)
		}

		return nil
	},
}

var invoicesMarkSentCmd = &cobra.Command{
	Use:   "mark-sent [id]",
	Short: "Mark an invoice as sent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid invoice ID: %w", err)
		}

		if err := appInstance.InvoiceService.MarkSent(ctx, id); err != nil {
			return fmt.Errorf("failed to mark invoice as sent: %w", err)
		}

		fmt.Printf("✓ Invoice #%d marked as sent\n", id)
		return nil
	},
}

var invoicesMarkPaidCmd = &cobra.Command{
	Use:   "mark-paid [id]",
	Short: "Mark an invoice as paid",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid invoice ID: %w", err)
		}

		// Parse paid date
		dateStr, _ := cmd.Flags().GetString("date")
		paidDate := time.Now()
		if dateStr != "" {
			var err error
			paidDate, err = parseDate(dateStr)
			if err != nil {
				return fmt.Errorf("invalid paid date: %w", err)
			}
		}

		if err := appInstance.InvoiceService.MarkPaid(ctx, id, paidDate); err != nil {
			return fmt.Errorf("failed to mark invoice as paid: %w", err)
		}

		fmt.Printf("✓ Invoice #%d marked as paid on %s\n", id, paidDate.Format("2006-01-02"))
		return nil
	},
}

var invoicesShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show invoice details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid invoice ID: %w", err)
		}

		invoice, err := appInstance.InvoiceService.GetInvoice(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get invoice: %w", err)
		}
		if invoice == nil {
			return fmt.Errorf("invoice not found")
		}

		// Load line items
		lineItems, err := appInstance.InvoiceRepo.GetLineItems(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to load line items: %w", err)
		}

		// Get client
		client, _ := appInstance.ClientRepo.GetByID(ctx, invoice.ClientID)
		clientName := fmt.Sprintf("Client #%d", invoice.ClientID)
		if client != nil {
			clientName = client.Name
		}

		// Print invoice details
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("Invoice: %s\n", invoice.InvoiceNumber)
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("Client: %s\n", clientName)
		fmt.Printf("Period: %s to %s\n",
			invoice.PeriodStart.Format("2006-01-02"),
			invoice.PeriodEnd.Format("2006-01-02"),
		)
		fmt.Printf("Status: %s\n", invoice.Status)
		fmt.Println()

		// Print line items
		if len(lineItems) > 0 {
			fmt.Println("Line Items:")
			fmt.Println(strings.Repeat("-", 80))
			fmt.Printf("%-12s %-40s %-8s %-8s %s\n", "Date", "Description", "Hours", "Rate", "Amount")
			fmt.Println(strings.Repeat("-", 80))

			for _, item := range lineItems {
				fmt.Printf("%-12s %-40s %8.2f $%7.2f $%8.2f\n",
					item.Date.Format("2006-01-02"),
					truncate(item.Description, 40),
					item.Hours,
					item.Rate,
					item.Amount,
				)
			}
			fmt.Println(strings.Repeat("-", 80))
		}

		// Print totals
		fmt.Printf("\n")
		fmt.Printf("Subtotal: $%.2f\n", invoice.Subtotal)
		fmt.Printf("Tax (%.1f%%): $%.2f\n", invoice.TaxRate*100, invoice.TaxAmount)
		fmt.Printf("Total: $%.2f\n", invoice.Total)
		fmt.Println(strings.Repeat("=", 80))

		return nil
	},
}

var invoicesRemoveEntryCmd = &cobra.Command{
	Use:   "remove-entry [invoice_id] [entry_id]",
	Short: "Remove a time entry from a draft invoice",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		invoiceID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid invoice ID: %w", err)
		}

		entryID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid entry ID: %w", err)
		}

		if err := appInstance.InvoiceService.RemoveEntryFromInvoice(ctx, invoiceID, entryID); err != nil {
			return fmt.Errorf("failed to remove entry from invoice: %w", err)
		}

		fmt.Printf("✓ Removed entry %d from invoice %d\n", entryID, invoiceID)
		// Show updated invoice totals
		invoice, _ := appInstance.InvoiceService.GetInvoice(ctx, invoiceID)
		if invoice != nil {
			fmt.Printf("  Subtotal: $%.2f\n", invoice.Subtotal)
			fmt.Printf("  Tax: $%.2f\n", invoice.TaxAmount)
			fmt.Printf("  Total: $%.2f\n", invoice.Total)
		}

		return nil
	},
}

func init() {
	invoicesCmd.AddCommand(invoicesListCmd)
	invoicesCmd.AddCommand(invoicesCreateCmd)
	invoicesCmd.AddCommand(invoicesAddEntriesCmd)
	invoicesCmd.AddCommand(invoicesFinalizeCmd)
	invoicesCmd.AddCommand(invoicesMarkSentCmd)
	invoicesCmd.AddCommand(invoicesMarkPaidCmd)
	invoicesCmd.AddCommand(invoicesShowCmd)
	invoicesCmd.AddCommand(invoicesRemoveEntryCmd)

	// List flags
	invoicesListCmd.Flags().Int64("client", 0, "Filter by client ID")
	invoicesListCmd.Flags().String("status", "", "Filter by status (draft, finalized, sent, paid, overdue)")

	// Create flags
	invoicesCreateCmd.Flags().String("start", "", "Period start date (required)")
	invoicesCreateCmd.Flags().String("end", "", "Period end date (required)")
	invoicesCreateCmd.Flags().String("prefix", "INV", "Invoice number prefix")
	invoicesCreateCmd.MarkFlagRequired("start")
	invoicesCreateCmd.MarkFlagRequired("end")

	// Add entries flags
	invoicesAddEntriesCmd.Flags().Float64("tax", 0, "Tax rate (0.0 to 1.0)")

	// Mark paid flags
	invoicesMarkPaidCmd.Flags().String("date", "", "Payment date (defaults to today)")
}
