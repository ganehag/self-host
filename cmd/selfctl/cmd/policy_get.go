package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var policyGetFormat string

var policyGetCmd = &cobra.Command{
	Use:   "get POLICY_UUID",
	Short: "Get one policy through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPolicyGet(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	policyCmd.AddCommand(policyGetCmd)
	policyGetCmd.Flags().StringVar(&policyGetFormat, "format", outputFormatTable, "Output format: table or json")
}

func runPolicyGet(id string) error {
	if err := validateDatasetOutputFormat(policyGetFormat); err != nil {
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
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.FindPolicyByUuidWithResponse(context.Background(), rest.UuidParam(policyID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("get policy failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printPolicy(resp.JSON200, policyGetFormat)
}
