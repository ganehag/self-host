package cmd

import "github.com/spf13/cobra"

var userPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Interact with user policy views through the Self-host API",
}

func init() {
	userCmd.AddCommand(userPolicyCmd)
}
