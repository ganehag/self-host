package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	groupListLimit  int64
	groupListOffset int64
	groupListFormat string
)

var groupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List groups through the Self-host API",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGroupList(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	groupCmd.AddCommand(groupListCmd)
	groupListCmd.Flags().Int64Var(&groupListLimit, "limit", 20, "Maximum number of groups to list")
	groupListCmd.Flags().Int64Var(&groupListOffset, "offset", 0, "Offset into the group list")
	groupListCmd.Flags().StringVar(&groupListFormat, "format", outputFormatTable, "Output format: table or json")
}

func runGroupList() error {
	if err := validateDatasetOutputFormat(groupListFormat); err != nil {
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
	params := &rest.FindGroupsParams{}
	if groupListLimit > 0 {
		v := rest.Limit(groupListLimit)
		params.Limit = &v
	}
	if groupListOffset > 0 {
		v := rest.Offset(groupListOffset)
		params.Offset = &v
	}
	resp, err := client.FindGroupsWithResponse(context.Background(), params)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("list groups failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printGroups(*resp.JSON200, groupListFormat)
}
