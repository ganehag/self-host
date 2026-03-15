package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var userPolicyListFormat string

var userPolicyListCmd = &cobra.Command{
	Use:   "list USER_UUID",
	Short: "List policies visible to a user",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUserPolicyList(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	userPolicyCmd.AddCommand(userPolicyListCmd)
	userPolicyListCmd.Flags().StringVar(&userPolicyListFormat, "format", outputFormatTable, "Output format: table or json")
}

func runUserPolicyList(id string) error {
	if err := validateDatasetOutputFormat(userPolicyListFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection("", "", "")
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
	resp, err := client.FindPoliciesForUserWithResponse(context.Background(), rest.UuidParam(userID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("list user policies failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printPolicies(*resp.JSON200, userPolicyListFormat)
}
