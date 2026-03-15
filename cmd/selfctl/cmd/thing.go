package cmd

import (
	"github.com/self-host/self-host/pkg/util/templates"
	"github.com/spf13/cobra"
)

var (
	thingCmdLong = templates.LongDesc(`
		Interact with things through the Self-host API
	`)
)

var thingCmd = &cobra.Command{
	Use:   "thing",
	Short: "Interact with things through the Self-host API",
	Long:  thingCmdLong,
}

func init() {
	rootCmd.AddCommand(thingCmd)
}
