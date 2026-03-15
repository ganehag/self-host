package cmd

import (
	"github.com/self-host/self-host/pkg/util/templates"
	"github.com/spf13/cobra"
)

var (
	configCmdLong = templates.LongDesc(`
		Manage local selfctl configuration defaults
	`)
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage local selfctl configuration defaults",
	Long:  configCmdLong,
}

func init() {
	rootCmd.AddCommand(configCmd)
}
