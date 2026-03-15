package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	userListServer string
	userListDomain string
	userListToken  string
	userListLimit  int64
	userListOffset int64
	userListFormat string
)

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List users through the Self-host API",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUserList(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	userCmd.AddCommand(userListCmd)
	userListCmd.Flags().Int64Var(&userListLimit, "limit", 20, "Maximum number of users to list")
	userListCmd.Flags().Int64Var(&userListOffset, "offset", 0, "Offset into the user list")
	userListCmd.Flags().StringVar(&userListFormat, "format", outputFormatTable, "Output format: table or json")
}

func runUserList() error {
	if err := validateDatasetOutputFormat(userListFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection(userListServer, userListDomain, userListToken)
	if err != nil {
		return err
	}

	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}

	params := &rest.FindUsersParams{}
	if userListLimit > 0 {
		v := rest.Limit(userListLimit)
		params.Limit = &v
	}
	if userListOffset > 0 {
		v := rest.Offset(userListOffset)
		params.Offset = &v
	}

	resp, err := client.FindUsersWithResponse(context.Background(), params)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("list users failed: %s", responseError(resp.StatusCode(), resp.Body))
	}

	return printUsers(*resp.JSON200, userListFormat)
}
