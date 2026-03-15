package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	userGetServer string
	userGetDomain string
	userGetToken  string
	userGetFormat string
)

var userGetCmd = &cobra.Command{
	Use:   "get USER_UUID",
	Short: "Get one user through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUserGet(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	userCmd.AddCommand(userGetCmd)
	userGetCmd.Flags().StringVar(&userGetFormat, "format", outputFormatTable, "Output format: table or json")
}

func runUserGet(id string) error {
	if err := validateDatasetOutputFormat(userGetFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection(userGetServer, userGetDomain, userGetToken)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid user uuid %q", id)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.FindUserByUuidWithResponse(context.Background(), rest.UuidParam(userID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("get user failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printUser(resp.JSON200, userGetFormat)
}
