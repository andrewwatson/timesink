package cli

import (
	"github.com/andy/timesink/internal/app"
	"github.com/spf13/cobra"
)

var appInstance *app.App

var rootCmd = &cobra.Command{
	Use:   "timesink",
	Short: "A CLI time tracking tool for freelancers",
	Long: `Timesink helps freelancers track time, manage clients, and generate invoices.

By default, running timesink without arguments launches the interactive TUI.
Use subcommands for CLI operations.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior: launch TUI
		launchTUI(cmd, args)
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// SetApp sets the app instance for commands to use
func SetApp(a *app.App) {
	appInstance = a
}

func init() {
	// Add all subcommands
	rootCmd.AddCommand(timerCmd)
	rootCmd.AddCommand(clientsCmd)
	rootCmd.AddCommand(entriesCmd)
	rootCmd.AddCommand(invoicesCmd)
	rootCmd.AddCommand(tuiCmd)
}
