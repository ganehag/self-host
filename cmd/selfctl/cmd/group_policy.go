package cmd

import "github.com/spf13/cobra"

var groupPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Interact with group policy views through the Self-host API",
}

func init() {
	groupCmd.AddCommand(groupPolicyCmd)
}
