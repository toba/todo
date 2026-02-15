package cmd

import (
	"fmt"

	"github.com/toba/todo/internal/version"
	"github.com/spf13/cobra"
)

// Set via ldflags at build time
var (
	ver    = "dev"
	commit = "unknown"
	date   = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("todo %s (%s) built %s [build %s]\n", ver, commit, date, version.Build)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
