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
	userTokenCreateServer string
	userTokenCreateDomain string
	userTokenCreateToken  string
	userTokenCreateFormat string
)

var userTokenCreateCmd = &cobra.Command{
	Use:   "create USER_UUID TOKEN_NAME",
	Short: "Create a new token for a user",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUserTokenCreate(args[0], args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	userTokenCmd.AddCommand(userTokenCreateCmd)
	userTokenCreateCmd.Flags().StringVar(&userTokenCreateFormat, "format", outputFormatTable, "Output format: table or json")
}

func runUserTokenCreate(id string, tokenName string) error {
	if err := validateDatasetOutputFormat(userTokenCreateFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection(userTokenCreateServer, userTokenCreateDomain, userTokenCreateToken)
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
	resp, err := client.AddNewTokenToUserWithResponse(context.Background(), rest.UuidParam(userID), rest.AddNewTokenToUserJSONRequestBody{
		Name: tokenName,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() != 201 || resp.JSON201 == nil {
		return fmt.Errorf("create user token failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printTokenWithSecret(resp.JSON201, userTokenCreateFormat)
}
