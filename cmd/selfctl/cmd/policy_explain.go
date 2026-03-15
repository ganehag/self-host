package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var policyExplainFormat string

var policyExplainCmd = &cobra.Command{
	Use:   "explain ACTION RESOURCE",
	Short: "Explain the current token's authorization decision",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPolicyExplain(args[0], args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	policyCmd.AddCommand(policyExplainCmd)
	policyExplainCmd.Flags().StringVar(&policyExplainFormat, "format", outputFormatTable, "Output format: table or json")
}

func runPolicyExplain(action, resource string) error {
	if err := validateDatasetOutputFormat(policyExplainFormat); err != nil {
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
	params := &rest.ExplainPolicyDecisionParams{
		Action:   rest.ActionParam(action),
		Resource: rest.ResourcePathParam(resource),
	}
	resp, err := client.ExplainPolicyDecisionWithResponse(context.Background(), params)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("explain policy failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printPolicyDecision(resp.JSON200, policyExplainFormat)
}
