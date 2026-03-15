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
	policyListLimit      int64
	policyListOffset     int64
	policyListGroupUUIDs []string
	policyListFormat     string
)

var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List policies through the Self-host API",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPolicyList(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	policyCmd.AddCommand(policyListCmd)
	policyListCmd.Flags().Int64Var(&policyListLimit, "limit", 20, "Maximum number of policies to list")
	policyListCmd.Flags().Int64Var(&policyListOffset, "offset", 0, "Offset into the policy list")
	policyListCmd.Flags().StringSliceVar(&policyListGroupUUIDs, "group-uuid", nil, "Filter policies by one or more group UUIDs")
	policyListCmd.Flags().StringVar(&policyListFormat, "format", outputFormatTable, "Output format: table or json")
}

func runPolicyList() error {
	if err := validateDatasetOutputFormat(policyListFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return err
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	params := &rest.FindPoliciesParams{}
	if policyListLimit > 0 {
		v := rest.Limit(policyListLimit)
		params.Limit = &v
	}
	if policyListOffset > 0 {
		v := rest.Offset(policyListOffset)
		params.Offset = &v
	}
	if len(policyListGroupUUIDs) > 0 {
		groupUUIDs := make(rest.GroupUUIDsFilterParam, 0, len(policyListGroupUUIDs))
		for _, id := range policyListGroupUUIDs {
			parsed, err := uuid.Parse(id)
			if err != nil {
				return fmt.Errorf("invalid group uuid %q", id)
			}
			groupUUIDs = append(groupUUIDs, parsed)
		}
		params.GroupUuids = &groupUUIDs
	}
	resp, err := client.FindPoliciesWithResponse(context.Background(), params)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("list policies failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printPolicies(*resp.JSON200, policyListFormat)
}
