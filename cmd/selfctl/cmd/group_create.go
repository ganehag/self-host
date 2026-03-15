package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	groupCreateName   string
	groupCreateFormat string
)

var groupCreateCmd = &cobra.Command{
	Use:   "create NAME",
	Short: "Create a group through the Self-host API",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGroupCreate(args); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	groupCmd.AddCommand(groupCreateCmd)
	groupCreateCmd.Flags().StringVar(&groupCreateName, "name", "", "Group name")
	groupCreateCmd.Flags().StringVar(&groupCreateFormat, "format", outputFormatTable, "Output format: table or json")
}

func runGroupCreate(args []string) error {
	if err := validateDatasetOutputFormat(groupCreateFormat); err != nil {
		return err
	}
	name := groupCreateName
	if len(args) == 1 {
		name = args[0]
	}
	if name == "" {
		return fmt.Errorf("group name is required")
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return err
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.AddGroupWithResponse(context.Background(), rest.AddGroupJSONRequestBody{Name: name})
	if err != nil {
		return err
	}
	if resp.StatusCode() != 201 || resp.JSON201 == nil {
		return fmt.Errorf("create group failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printGroup(resp.JSON201, groupCreateFormat)
}
