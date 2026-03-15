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
	userTokenListServer string
	userTokenListDomain string
	userTokenListToken  string
	userTokenListFormat string
)

var userTokenListCmd = &cobra.Command{
	Use:   "list USER_UUID",
	Short: "List tokens for a user",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUserTokenList(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	userTokenCmd.AddCommand(userTokenListCmd)
	userTokenListCmd.Flags().StringVar(&userTokenListFormat, "format", outputFormatTable, "Output format: table or json")
}

func runUserTokenList(id string) error {
	if err := validateDatasetOutputFormat(userTokenListFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection(userTokenListServer, userTokenListDomain, userTokenListToken)
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
	resp, err := client.FindTokensForUserWithResponse(context.Background(), rest.UuidParam(userID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("list user tokens failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printTokens(*resp.JSON200, userTokenListFormat)
}
