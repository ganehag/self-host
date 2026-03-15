package cmd

import (
	"github.com/self-host/self-host/pkg/util/templates"
	"github.com/spf13/cobra"
)

var (
	policyCmdLong = templates.LongDesc(`
		Interact with access-control policies through the Self-host API
	`)
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Interact with access-control policies through the Self-host API",
	Long:  policyCmdLong,
}

func init() {
	rootCmd.AddCommand(policyCmd)
}
