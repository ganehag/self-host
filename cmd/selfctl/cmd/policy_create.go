package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	policyCreateGroupUUID string
	policyCreatePriority  int
	policyCreateEffect    string
	policyCreateAction    string
	policyCreateResource  string
	policyCreateFormat    string
)

var policyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a policy through the Self-host API",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPolicyCreate(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	policyCmd.AddCommand(policyCreateCmd)
	policyCreateCmd.Flags().StringVar(&policyCreateGroupUUID, "group-uuid", "", "Group UUID")
	policyCreateCmd.Flags().IntVar(&policyCreatePriority, "priority", 100, "Policy priority; lower numbers are evaluated first")
	policyCreateCmd.Flags().StringVar(&policyCreateEffect, "effect", "", "Policy effect: allow or deny")
	policyCreateCmd.Flags().StringVar(&policyCreateAction, "action", "", "Policy action, for example read or write")
	policyCreateCmd.Flags().StringVar(&policyCreateResource, "resource", "", "Canonical resource pattern, for example timeseries/%")
	policyCreateCmd.Flags().StringVar(&policyCreateFormat, "format", outputFormatTable, "Output format: table or json")
}

func runPolicyCreate() error {
	if err := validateDatasetOutputFormat(policyCreateFormat); err != nil {
		return err
	}
	if policyCreateGroupUUID == "" || policyCreateEffect == "" || policyCreateAction == "" || policyCreateResource == "" {
		return fmt.Errorf("group-uuid, effect, action, and resource are required")
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return err
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	body := rest.AddPolicyJSONRequestBody{
		GroupUuid: policyCreateGroupUUID,
		Priority:  policyCreatePriority,
		Effect:    rest.AddPolicyJSONBodyEffect(policyCreateEffect),
		Action:    rest.AddPolicyJSONBodyAction(policyCreateAction),
		Resource:  policyCreateResource,
	}
	resp, err := client.AddPolicyWithResponse(context.Background(), body)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 201 || resp.JSON201 == nil {
		return fmt.Errorf("create policy failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printPolicy(resp.JSON201, policyCreateFormat)
}
