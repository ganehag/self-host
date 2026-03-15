package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var policyDeleteCmd = &cobra.Command{
	Use:   "delete POLICY_UUID",
	Short: "Delete a policy through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPolicyDelete(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	policyCmd.AddCommand(policyDeleteCmd)
}

func runPolicyDelete(id string) error {
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return err
	}
	policyID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid policy uuid %q", id)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.DeletePolicyByUuidWithResponse(context.Background(), rest.UuidParam(policyID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("delete policy failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	fmt.Fprintf(os.Stdout, "deleted policy %s\n", id)
	return nil
}
