package cmd

import (
	"github.com/self-host/self-host/pkg/util/templates"
	"github.com/spf13/cobra"
)

var (
	apiCmdLong = templates.LongDesc(`
		Make raw API requests against the configured Self-host API
	`)
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Make raw API requests against the configured Self-host API",
	Long:  apiCmdLong,
}

func init() {
	rootCmd.AddCommand(apiCmd)
}
