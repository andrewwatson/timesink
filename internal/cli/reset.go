package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset data in the database",
	Long: `Reset data in the database. By default resets only time entries.

Examples:
  timesink reset --entries    # Delete all time entries (and related invoice data)
  timesink reset --all        # Wipe everything: entries, clients, invoices, timers`,
}

var resetEntriesCmd = &cobra.Command{
	Use:   "entries",
	Short: "Delete all time entries, invoices, and timer state",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !confirmPrompt("This will delete ALL time entries, invoices, and timer state. Continue?") {
			fmt.Println("Cancelled.")
			return nil
		}

		db := appInstance.DB

		// Clear invoice references from entries before deleting invoices
		if _, err := db.Exec("UPDATE time_entries SET invoice_id = NULL WHERE invoice_id IS NOT NULL"); err != nil {
			return fmt.Errorf("failed to unlock entries: %w", err)
		}

		// Order matters due to foreign keys
		tables := []string{
			"invoice_line_items",
			"invoices",
			"entry_history",
			"time_entries",
			"active_timer",
		}

		for _, table := range tables {
			if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
				return fmt.Errorf("failed to clear %s: %w", table, err)
			}
		}

		fmt.Println("All time entries, invoices, and timer state have been deleted.")
		return nil
	},
}

var resetInvoicesCmd = &cobra.Command{
	Use:   "invoices",
	Short: "Delete all invoices and unlock associated time entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !confirmPrompt("This will delete ALL invoices and unlock all time entries. Continue?") {
			fmt.Println("Cancelled.")
			return nil
		}

		db := appInstance.DB

		// Unlock all entries that were locked to invoices
		if _, err := db.Exec("UPDATE time_entries SET invoice_id = NULL WHERE invoice_id IS NOT NULL"); err != nil {
			return fmt.Errorf("failed to unlock entries: %w", err)
		}

		tables := []string{
			"invoice_line_items",
			"invoices",
		}

		for _, table := range tables {
			if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
				return fmt.Errorf("failed to clear %s: %w", table, err)
			}
		}

		fmt.Println("All invoices have been deleted and time entries unlocked.")
		return nil
	},
}

var resetAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Delete ALL data: clients, entries, invoices, everything",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !confirmPrompt("This will delete ALL data (clients, entries, invoices, everything). Continue?") {
			fmt.Println("Cancelled.")
			return nil
		}

		db := appInstance.DB

		// Clear invoice references from entries before deleting invoices
		if _, err := db.Exec("UPDATE time_entries SET invoice_id = NULL WHERE invoice_id IS NOT NULL"); err != nil {
			return fmt.Errorf("failed to unlock entries: %w", err)
		}

		// Order matters due to foreign keys
		tables := []string{
			"invoice_line_items",
			"invoices",
			"entry_history",
			"time_entries",
			"active_timer",
			"clients",
		}

		for _, table := range tables {
			if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
				return fmt.Errorf("failed to clear %s: %w", table, err)
			}
		}

		fmt.Println("All data has been deleted.")
		return nil
	},
}

func confirmPrompt(message string) bool {
	fmt.Printf("%s [y/N] ", message)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func init() {
	resetCmd.AddCommand(resetEntriesCmd)
	resetCmd.AddCommand(resetInvoicesCmd)
	resetCmd.AddCommand(resetAllCmd)
}
