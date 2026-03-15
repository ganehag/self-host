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
	groupUpdateName   string
	groupUpdateFormat string
)

var groupUpdateCmd = &cobra.Command{
	Use:   "update GROUP_UUID NAME",
	Short: "Update a group through the Self-host API",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGroupUpdate(args); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	groupCmd.AddCommand(groupUpdateCmd)
	groupUpdateCmd.Flags().StringVar(&groupUpdateName, "name", "", "New group name")
	groupUpdateCmd.Flags().StringVar(&groupUpdateFormat, "format", outputFormatTable, "Output format: table or json")
}

func runGroupUpdate(args []string) error {
	if err := validateDatasetOutputFormat(groupUpdateFormat); err != nil {
		return err
	}
	groupID, err := uuid.Parse(args[0])
	if err != nil {
		return fmt.Errorf("invalid group uuid %q", args[0])
	}
	name := groupUpdateName
	if len(args) == 2 {
		name = args[1]
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
	resp, err := client.UpdateGroupByUuidWithResponse(context.Background(), rest.UuidParam(groupID), rest.UpdateGroupByUuidJSONRequestBody{Name: name})
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("update group failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	getResp, err := client.FindGroupByUuidWithResponse(context.Background(), rest.UuidParam(groupID))
	if err != nil {
		return err
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		return fmt.Errorf("updated group but failed to fetch it: %s", responseError(getResp.StatusCode(), getResp.Body))
	}
	return printGroup(getResp.JSON200, groupUpdateFormat)
}
