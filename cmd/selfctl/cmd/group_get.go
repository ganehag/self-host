package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var groupGetFormat string

var groupGetCmd = &cobra.Command{
	Use:   "get GROUP_UUID",
	Short: "Get one group through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGroupGet(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	groupCmd.AddCommand(groupGetCmd)
	groupGetCmd.Flags().StringVar(&groupGetFormat, "format", outputFormatTable, "Output format: table or json")
}

func runGroupGet(id string) error {
	if err := validateDatasetOutputFormat(groupGetFormat); err != nil {
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
	resp, err := client.FindGroupByUuidWithResponse(context.Background(), rest.UuidParam(groupID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("get group failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printGroup(resp.JSON200, groupGetFormat)
}
