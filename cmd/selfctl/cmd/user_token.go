package cmd

import "github.com/spf13/cobra"

var userTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage user tokens through the Self-host API",
}

func init() {
	userCmd.AddCommand(userTokenCmd)
}
