package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var timerCmd = &cobra.Command{
	Use:   "timer",
	Short: "Manage the active timer",
	Long:  `Start, stop, pause, resume, or check the status of the active timer.`,
}

var timerStartCmd = &cobra.Command{
	Use:   "start [client_id_or_name] [description]",
	Short: "Start a new timer",
	Long:  `Start a new timer for a client with an optional description.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Parse client ID or name
		clientID, err := resolveClientID(ctx, args[0])
		if err != nil {
			return fmt.Errorf("failed to resolve client: %w", err)
		}

		// Get description (everything after client)
		description := ""
		if len(args) > 1 {
			description = args[1]
		}

		// Start timer
		if err := appInstance.TimerService.Start(ctx, clientID, description); err != nil {
			return fmt.Errorf("failed to start timer: %w", err)
		}

		// Get client for display
		client, _ := appInstance.ClientRepo.GetByID(ctx, clientID)
		clientName := fmt.Sprintf("Client #%d", clientID)
		if client != nil {
			clientName = client.Name
		}

		fmt.Printf("✓ Timer started for %s\n", clientName)
		if description != "" {
			fmt.Printf("  Description: %s\n", description)
		}

		return nil
	},
}

var timerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the active timer and save the time entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		entry, err := appInstance.TimerService.Stop(ctx)
		if err != nil {
			return fmt.Errorf("failed to stop timer: %w", err)
		}

		// Get client for display
		client, _ := appInstance.ClientRepo.GetByID(ctx, entry.ClientID)
		clientName := fmt.Sprintf("Client #%d", entry.ClientID)
		if client != nil {
			clientName = client.Name
		}

		duration := entry.Duration()
		fmt.Printf("✓ Timer stopped\n")
		fmt.Printf("  Client: %s\n", clientName)
		fmt.Printf("  Duration: %s\n", formatDuration(duration))
		fmt.Printf("  Amount: $%.2f\n", entry.Amount())

		return nil
	},
}

var timerPauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the active timer",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if err := appInstance.TimerService.Pause(ctx); err != nil {
			return fmt.Errorf("failed to pause timer: %w", err)
		}

		fmt.Println("✓ Timer paused")
		return nil
	},
}

var timerResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a paused timer",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if err := appInstance.TimerService.Resume(ctx); err != nil {
			return fmt.Errorf("failed to resume timer: %w", err)
		}

		fmt.Println("✓ Timer resumed")
		return nil
	},
}

var timerDiscardCmd = &cobra.Command{
	Use:   "discard",
	Short: "Discard the active timer without saving",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if err := appInstance.TimerService.Discard(ctx); err != nil {
			return fmt.Errorf("failed to discard timer: %w", err)
		}

		fmt.Println("✓ Timer discarded")
		return nil
	},
}

var timerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the active timer",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		state, err := appInstance.TimerService.GetState(ctx)
		if err != nil {
			return fmt.Errorf("failed to get timer state: %w", err)
		}

		if state == "idle" {
			fmt.Println("No active timer")
			return nil
		}

		timer, err := appInstance.TimerService.GetActiveTimer(ctx)
		if err != nil {
			return fmt.Errorf("failed to get active timer: %w", err)
		}

		// Get client for display
		client, _ := appInstance.ClientRepo.GetByID(ctx, timer.ClientID)
		clientName := fmt.Sprintf("Client #%d", timer.ClientID)
		if client != nil {
			clientName = client.Name
		}

		elapsed := timer.Elapsed()
		value := elapsed.Hours() * client.HourlyRate

		fmt.Printf("Timer Status: %s\n", state)
		fmt.Printf("  Client: %s\n", clientName)
		if timer.Description != "" {
			fmt.Printf("  Description: %s\n", timer.Description)
		}
		fmt.Printf("  Started: %s\n", timer.StartTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Elapsed: %s\n", formatDuration(elapsed))
		fmt.Printf("  Current Value: $%.2f\n", value)

		return nil
	},
}

func init() {
	timerCmd.AddCommand(timerStartCmd)
	timerCmd.AddCommand(timerStopCmd)
	timerCmd.AddCommand(timerPauseCmd)
	timerCmd.AddCommand(timerResumeCmd)
	timerCmd.AddCommand(timerDiscardCmd)
	timerCmd.AddCommand(timerStatusCmd)
}

// resolveClientID resolves a client by ID or name
func resolveClientID(ctx context.Context, idOrName string) (int64, error) {
	// Try to parse as ID first
	if id, err := strconv.ParseInt(idOrName, 10, 64); err == nil {
		// Verify client exists
		client, err := appInstance.ClientRepo.GetByID(ctx, id)
		if err != nil {
			return 0, err
		}
		if client == nil {
			return 0, fmt.Errorf("client with ID %d not found", id)
		}
		return id, nil
	}

	// Try to find by name
	client, err := appInstance.ClientRepo.GetByName(ctx, idOrName)
	if err != nil {
		return 0, err
	}
	if client == nil {
		return 0, fmt.Errorf("client named '%s' not found", idOrName)
	}

	return client.ID, nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
