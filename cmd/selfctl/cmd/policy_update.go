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
	policyUpdateGroupUUID string
	policyUpdatePriority  int
	policyUpdateEffect    string
	policyUpdateAction    string
	policyUpdateResource  string
	policyUpdateFormat    string
)

var policyUpdateCmd = &cobra.Command{
	Use:   "update POLICY_UUID",
	Short: "Update a policy through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPolicyUpdate(args[0], cmd); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	policyCmd.AddCommand(policyUpdateCmd)
	policyUpdateCmd.Flags().StringVar(&policyUpdateGroupUUID, "group-uuid", "", "New group UUID")
	policyUpdateCmd.Flags().IntVar(&policyUpdatePriority, "priority", 0, "New policy priority")
	policyUpdateCmd.Flags().StringVar(&policyUpdateEffect, "effect", "", "New policy effect")
	policyUpdateCmd.Flags().StringVar(&policyUpdateAction, "action", "", "New policy action")
	policyUpdateCmd.Flags().StringVar(&policyUpdateResource, "resource", "", "New resource pattern")
	policyUpdateCmd.Flags().StringVar(&policyUpdateFormat, "format", outputFormatTable, "Output format: table or json")
}

func runPolicyUpdate(id string, cmd *cobra.Command) error {
	if err := validateDatasetOutputFormat(policyUpdateFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return err
	}
	policyID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid policy uuid %q", id)
	}
	body := rest.UpdatePolicyByUuidJSONRequestBody{}
	if policyUpdateGroupUUID != "" {
		body.GroupUuid = &policyUpdateGroupUUID
	}
	if flag := cmd.Flags().Lookup("priority"); flag != nil && flag.Changed {
		body.Priority = &policyUpdatePriority
	}
	if policyUpdateEffect != "" {
		effect := rest.UpdatePolicyByUuidJSONBodyEffect(policyUpdateEffect)
		body.Effect = &effect
	}
	if policyUpdateAction != "" {
		action := rest.UpdatePolicyByUuidJSONBodyAction(policyUpdateAction)
		body.Action = &action
	}
	if policyUpdateResource != "" {
		body.Resource = &policyUpdateResource
	}
	if body.GroupUuid == nil && body.Priority == nil && body.Effect == nil && body.Action == nil && body.Resource == nil {
		return fmt.Errorf("no update fields provided")
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.UpdatePolicyByUuidWithResponse(context.Background(), rest.UuidParam(policyID), body)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("update policy failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	getResp, err := client.FindPolicyByUuidWithResponse(context.Background(), rest.UuidParam(policyID))
	if err != nil {
		return err
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		return fmt.Errorf("updated policy but failed to fetch it: %s", responseError(getResp.StatusCode(), getResp.Body))
	}
	return printPolicy(getResp.JSON200, policyUpdateFormat)
}
