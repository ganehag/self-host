package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var groupPolicyListFormat string

var groupPolicyListCmd = &cobra.Command{
	Use:   "list GROUP_UUID",
	Short: "List policies attached to a group",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGroupPolicyList(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	groupPolicyCmd.AddCommand(groupPolicyListCmd)
	groupPolicyListCmd.Flags().StringVar(&groupPolicyListFormat, "format", outputFormatTable, "Output format: table or json")
}

func runGroupPolicyList(id string) error {
	if err := validateDatasetOutputFormat(groupPolicyListFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return err
	}
	groupID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid group uuid %q", id)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.FindPoliciesForGroupWithResponse(context.Background(), rest.UuidParam(groupID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("list group policies failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printPolicies(*resp.JSON200, groupPolicyListFormat)
}
