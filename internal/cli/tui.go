package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the terminal UI",
	Long:  `Launch the interactive terminal user interface for timesink.`,
	Run:   launchTUI,
}

func launchTUI(cmd *cobra.Command, args []string) {
	// TODO: Implement TUI launcher once TUI package is ready
	fmt.Println("TUI not yet implemented - coming soon!")
	fmt.Println()
	fmt.Println("For now, use CLI commands:")
	fmt.Println("  timesink timer start <client> <description>")
	fmt.Println("  timesink timer stop")
	fmt.Println("  timesink clients list")
	fmt.Println("  timesink entries list")
	fmt.Println("  timesink invoices list")
	fmt.Println()
	fmt.Println("Run 'timesink --help' for more commands")
}
