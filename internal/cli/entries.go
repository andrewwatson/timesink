package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/andy/timesink/internal/domain"
	"github.com/spf13/cobra"
)

var entriesCmd = &cobra.Command{
	Use:   "entries",
	Short: "Manage time entries",
	Long:  `List, add, edit, and delete time entries.`,
}

var entriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List time entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Parse filters
		var clientID *int64
		if cmd.Flags().Changed("client") {
			id, _ := cmd.Flags().GetInt64("client")
			clientID = &id
		}

		var start, end *time.Time
		if cmd.Flags().Changed("start") {
			startStr, _ := cmd.Flags().GetString("start")
			t, err := parseDate(startStr)
			if err != nil {
				return fmt.Errorf("invalid start date: %w", err)
			}
			start = &t
		}
		if cmd.Flags().Changed("end") {
			endStr, _ := cmd.Flags().GetString("end")
			t, err := parseDate(endStr)
			if err != nil {
				return fmt.Errorf("invalid end date: %w", err)
			}
			end = &t
		}

		includeLocked, _ := cmd.Flags().GetBool("include-locked")

		entries, err := appInstance.EntryRepo.List(ctx, clientID, start, end, includeLocked)
		if err != nil {
			return fmt.Errorf("failed to list entries: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("No entries found")
			return nil
		}

		// Print table header
		fmt.Printf("%-5s %-15s %-20s %-10s %-12s %-8s\n", "ID", "Client", "Date", "Duration", "Amount", "Status")
		fmt.Println("--------------------------------------------------------------------------------")

		var totalDuration time.Duration
		var totalAmount float64

		// Print entries
		for _, entry := range entries {
			client, _ := appInstance.ClientRepo.GetByID(ctx, entry.ClientID)
			clientName := fmt.Sprintf("Client #%d", entry.ClientID)
			if client != nil {
				clientName = client.Name
			}

			status := "Unbilled"
			if entry.InvoiceID != nil {
				status = "Invoiced"
			}

			duration := entry.Duration()
			amount := entry.Amount()

			fmt.Printf("%-5d %-15s %-20s %-10s $%-11.2f %-8s\n",
				entry.ID,
				truncate(clientName, 15),
				entry.StartTime.Format("2006-01-02 15:04"),
				formatDuration(duration),
				amount,
				status,
			)

			totalDuration += duration
			totalAmount += amount
		}

		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Printf("Total: %d entries, %s, $%.2f\n", len(entries), formatDuration(totalDuration), totalAmount)
		return nil
	},
}

var entriesAddCmd = &cobra.Command{
	Use:   "add [client_id_or_name] [start_time] [end_time] [description]",
	Short: "Add a time entry manually",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Resolve client
		clientID, err := resolveClientID(ctx, args[0])
		if err != nil {
			return fmt.Errorf("failed to resolve client: %w", err)
		}

		// Parse times
		startTime, err := parseDateTime(args[1])
		if err != nil {
			return fmt.Errorf("invalid start time: %w", err)
		}

		endTime, err := parseDateTime(args[2])
		if err != nil {
			return fmt.Errorf("invalid end time: %w", err)
		}

		// Get description
		description := ""
		if len(args) > 3 {
			description = args[3]
		}

		// Get rate (use flag or client's rate)
		client, err := appInstance.ClientRepo.GetByID(ctx, clientID)
		if err != nil {
			return fmt.Errorf("failed to get client: %w", err)
		}
		if client == nil {
			return fmt.Errorf("client not found")
		}

		rate := client.HourlyRate
		if cmd.Flags().Changed("rate") {
			rate, _ = cmd.Flags().GetFloat64("rate")
		}

		// Create entry
		entry := domain.NewTimeEntry(clientID, description, rate)
		entry.StartTime = startTime
		entry.Stop(endTime)

		if err := entry.Validate(); err != nil {
			return fmt.Errorf("invalid entry: %w", err)
		}

		if err := appInstance.EntryRepo.Create(ctx, entry); err != nil {
			return fmt.Errorf("failed to create entry: %w", err)
		}

		duration := entry.Duration()
		fmt.Printf("✓ Time entry created (ID: %d)\n", entry.ID)
		fmt.Printf("  Client: %s\n", client.Name)
		fmt.Printf("  Duration: %s\n", formatDuration(duration))
		fmt.Printf("  Amount: $%.2f\n", entry.Amount())

		return nil
	},
}

var entriesEditCmd = &cobra.Command{
	Use:   "edit [id]",
	Short: "Edit a time entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid entry ID: %w", err)
		}

		entry, err := appInstance.EntryRepo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get entry: %w", err)
		}
		if entry == nil {
			return fmt.Errorf("entry not found")
		}

		// Check if locked
		if entry.IsLocked() {
			return fmt.Errorf("cannot edit entry: already invoiced")
		}

		// Update fields if flags provided
		if cmd.Flags().Changed("description") {
			description, _ := cmd.Flags().GetString("description")
			entry.Description = description
		}

		reason, _ := cmd.Flags().GetString("reason")
		if reason == "" {
			return fmt.Errorf("--reason flag is required for editing entries")
		}

		if err := entry.Validate(); err != nil {
			return fmt.Errorf("invalid entry: %w", err)
		}

		if err := appInstance.EntryRepo.Update(ctx, entry, reason); err != nil {
			return fmt.Errorf("failed to update entry: %w", err)
		}

		fmt.Printf("✓ Entry updated (ID: %d)\n", entry.ID)
		return nil
	},
}

var entriesDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a time entry (soft delete)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid entry ID: %w", err)
		}

		reason, _ := cmd.Flags().GetString("reason")
		if reason == "" {
			return fmt.Errorf("--reason flag is required for deleting entries")
		}

		if err := appInstance.EntryRepo.SoftDelete(ctx, id, reason); err != nil {
			return fmt.Errorf("failed to delete entry: %w", err)
		}

		fmt.Printf("✓ Entry deleted (ID: %d)\n", id)
		return nil
	},
}

var entriesHistoryCmd = &cobra.Command{
	Use:   "history [id]",
	Short: "Show edit history for an entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid entry ID: %w", err)
		}

		history, err := appInstance.EntryRepo.GetHistory(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get history: %w", err)
		}

		if len(history) == 0 {
			fmt.Println("No edit history for this entry")
			return nil
		}

		fmt.Printf("Edit History for Entry #%d:\n\n", id)
		for _, h := range history {
			fmt.Printf("%s - %s\n", h.ChangedAt.Format("2006-01-02 15:04:05"), h.FieldName)
			if h.ChangeReason != "" {
				fmt.Printf("  Reason: %s\n", h.ChangeReason)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	entriesCmd.AddCommand(entriesListCmd)
	entriesCmd.AddCommand(entriesAddCmd)
	entriesCmd.AddCommand(entriesEditCmd)
	entriesCmd.AddCommand(entriesDeleteCmd)
	entriesCmd.AddCommand(entriesHistoryCmd)

	// List flags
	entriesListCmd.Flags().Int64("client", 0, "Filter by client ID")
	entriesListCmd.Flags().String("start", "", "Filter by start date (YYYY-MM-DD or 'today')")
	entriesListCmd.Flags().String("end", "", "Filter by end date (YYYY-MM-DD or 'today')")
	entriesListCmd.Flags().Bool("include-locked", false, "Include invoiced entries")

	// Add flags
	entriesAddCmd.Flags().Float64("rate", 0, "Override hourly rate")

	// Edit flags
	entriesEditCmd.Flags().String("description", "", "New description")
	entriesEditCmd.Flags().String("reason", "", "Reason for edit (required)")

	// Delete flags
	entriesDeleteCmd.Flags().String("reason", "", "Reason for deletion (required)")
}

// parseDate parses a date string in various formats
func parseDate(s string) (time.Time, error) {
	switch s {
	case "today":
		return time.Now().Truncate(24 * time.Hour), nil
	case "yesterday":
		return time.Now().Add(-24 * time.Hour).Truncate(24 * time.Hour), nil
	default:
		// Try YYYY-MM-DD format
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}, fmt.Errorf("expected format: YYYY-MM-DD, 'today', or 'yesterday'")
		}
		return t, nil
	}
}

// parseDateTime parses a datetime string in various formats
func parseDateTime(s string) (time.Time, error) {
	// Try ISO format with time
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t, nil
	}

	// Try date + space + time
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}

	// Try date + space + time (no seconds)
	if t, err := time.Parse("2006-01-02 15:04", s); err == nil {
		return t, nil
	}

	// Try just date (assume midnight)
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("expected format: YYYY-MM-DD or YYYY-MM-DD HH:MM:SS")
}
