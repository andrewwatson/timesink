package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/andy/timesink/internal/domain"
	"github.com/spf13/cobra"
)

var clientsCmd = &cobra.Command{
	Use:   "clients",
	Short: "Manage clients",
	Long:  `List, add, edit, and archive clients.`,
}

var clientsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all clients",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		includeArchived, _ := cmd.Flags().GetBool("archived")

		clients, err := appInstance.ClientRepo.List(ctx, includeArchived)
		if err != nil {
			return fmt.Errorf("failed to list clients: %w", err)
		}

		if len(clients) == 0 {
			fmt.Println("No clients found")
			return nil
		}

		// Print table header
		fmt.Printf("%-5s %-30s %-15s %-10s\n", "ID", "Name", "Hourly Rate", "Status")
		fmt.Println("----------------------------------------------------------------------")

		// Print clients
		for _, client := range clients {
			status := "Active"
			if client.IsArchived {
				status = "Archived"
			}
			fmt.Printf("%-5d %-30s $%-14.2f %-10s\n",
				client.ID,
				truncate(client.Name, 30),
				client.HourlyRate,
				status,
			)
		}

		fmt.Printf("\nTotal: %d client(s)\n", len(clients))
		return nil
	},
}

var clientsAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a new client",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		rate, _ := cmd.Flags().GetFloat64("rate")
		email, _ := cmd.Flags().GetString("email")
		notes, _ := cmd.Flags().GetString("notes")

		client := domain.NewClient(name, rate)
		client.Email = email
		client.Notes = notes

		if err := client.Validate(); err != nil {
			return fmt.Errorf("invalid client: %w", err)
		}

		if err := appInstance.ClientRepo.Create(ctx, client); err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		fmt.Printf("✓ Client created: %s (ID: %d)\n", client.Name, client.ID)
		fmt.Printf("  Hourly Rate: $%.2f\n", client.HourlyRate)

		return nil
	},
}

var clientsEditCmd = &cobra.Command{
	Use:   "edit [id]",
	Short: "Edit an existing client",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid client ID: %w", err)
		}

		client, err := appInstance.ClientRepo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get client: %w", err)
		}
		if client == nil {
			return fmt.Errorf("client not found")
		}

		// Update fields if flags provided
		if cmd.Flags().Changed("name") {
			name, _ := cmd.Flags().GetString("name")
			client.Name = name
		}
		if cmd.Flags().Changed("rate") {
			rate, _ := cmd.Flags().GetFloat64("rate")
			client.HourlyRate = rate
		}
		if cmd.Flags().Changed("email") {
			email, _ := cmd.Flags().GetString("email")
			client.Email = email
		}
		if cmd.Flags().Changed("notes") {
			notes, _ := cmd.Flags().GetString("notes")
			client.Notes = notes
		}

		if err := client.Validate(); err != nil {
			return fmt.Errorf("invalid client: %w", err)
		}

		if err := appInstance.ClientRepo.Update(ctx, client); err != nil {
			return fmt.Errorf("failed to update client: %w", err)
		}

		fmt.Printf("✓ Client updated: %s\n", client.Name)
		return nil
	},
}

var clientsArchiveCmd = &cobra.Command{
	Use:   "archive [id]",
	Short: "Archive a client",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid client ID: %w", err)
		}

		client, err := appInstance.ClientRepo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get client: %w", err)
		}
		if client == nil {
			return fmt.Errorf("client not found")
		}

		if err := appInstance.ClientRepo.Archive(ctx, id); err != nil {
			return fmt.Errorf("failed to archive client: %w", err)
		}

		fmt.Printf("✓ Client archived: %s\n", client.Name)
		return nil
	},
}

var clientsUnarchiveCmd = &cobra.Command{
	Use:   "unarchive [id]",
	Short: "Unarchive a client",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid client ID: %w", err)
		}

		if err := appInstance.ClientRepo.Unarchive(ctx, id); err != nil {
			return fmt.Errorf("failed to unarchive client: %w", err)
		}

		fmt.Printf("✓ Client unarchived (ID: %d)\n", id)
		return nil
	},
}

func init() {
	clientsCmd.AddCommand(clientsListCmd)
	clientsCmd.AddCommand(clientsAddCmd)
	clientsCmd.AddCommand(clientsEditCmd)
	clientsCmd.AddCommand(clientsArchiveCmd)
	clientsCmd.AddCommand(clientsUnarchiveCmd)

	// List flags
	clientsListCmd.Flags().Bool("archived", false, "Include archived clients")

	// Add flags
	clientsAddCmd.Flags().Float64("rate", 0, "Hourly rate (required)")
	clientsAddCmd.MarkFlagRequired("rate")
	clientsAddCmd.Flags().String("email", "", "Client email")
	clientsAddCmd.Flags().String("notes", "", "Notes about the client")

	// Edit flags
	clientsEditCmd.Flags().String("name", "", "New name")
	clientsEditCmd.Flags().Float64("rate", 0, "New hourly rate")
	clientsEditCmd.Flags().String("email", "", "New email")
	clientsEditCmd.Flags().String("notes", "", "New notes")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
