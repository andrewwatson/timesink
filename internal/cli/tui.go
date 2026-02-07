package cli

import (
	"fmt"
	"os"

	"github.com/andy/timesink/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the terminal UI",
	Long:  `Launch the interactive terminal user interface for timesink.`,
	Run:   launchTUI,
}

func launchTUI(cmd *cobra.Command, args []string) {
	if err := tui.Run(appInstance); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
