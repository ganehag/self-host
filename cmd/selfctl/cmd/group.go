package cmd

import (
	"github.com/self-host/self-host/pkg/util/templates"
	"github.com/spf13/cobra"
)

var (
	groupCmdLong = templates.LongDesc(`
		Interact with groups through the Self-host API
	`)
)

var groupCmd = &cobra.Command{
	Use:   "group",
	Short: "Interact with groups through the Self-host API",
	Long:  groupCmdLong,
}

func init() {
	rootCmd.AddCommand(groupCmd)
}
