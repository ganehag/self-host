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
	thingUpdateName   string
	thingUpdateType   string
	thingUpdateState  string
	thingUpdateTags   []string
	thingUpdateFormat string
)

var thingUpdateCmd = &cobra.Command{
	Use:   "update THING_UUID",
	Short: "Update a thing through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runThingUpdate(args[0], cmd); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	thingCmd.AddCommand(thingUpdateCmd)
	thingUpdateCmd.Flags().StringVar(&thingUpdateName, "name", "", "New thing name")
	thingUpdateCmd.Flags().StringVar(&thingUpdateType, "thing-type", "", "New thing type")
	thingUpdateCmd.Flags().StringVar(&thingUpdateState, "state", "", "New thing state")
	thingUpdateCmd.Flags().StringSliceVar(&thingUpdateTags, "tags", nil, "Replace thing tags")
	thingUpdateCmd.Flags().StringVar(&thingUpdateFormat, "format", outputFormatTable, "Output format: table or json")
}

func runThingUpdate(id string, cmd *cobra.Command) error {
	if err := validateDatasetOutputFormat(thingUpdateFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return err
	}
	thingID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid thing uuid %q", id)
	}
	body := rest.UpdateThingByUuidJSONRequestBody{}
	if thingUpdateName != "" {
		body.Name = &thingUpdateName
	}
	if thingUpdateType != "" {
		body.Type = &thingUpdateType
	}
	if thingUpdateState != "" {
		state := rest.UpdateThingByUuidJSONBodyState(thingUpdateState)
		body.State = &state
	}
	if flag := cmd.Flags().Lookup("tags"); flag != nil && flag.Changed {
		tags := append([]string(nil), thingUpdateTags...)
		body.Tags = &tags
	}
	if body.Name == nil && body.Type == nil && body.State == nil && body.Tags == nil {
		return fmt.Errorf("no update fields provided")
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.UpdateThingByUuidWithResponse(context.Background(), rest.UuidParam(thingID), body)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("update thing failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	getResp, err := client.FindThingByUuidWithResponse(context.Background(), rest.UuidParam(thingID))
	if err != nil {
		return err
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		return fmt.Errorf("updated thing but failed to fetch it: %s", responseError(getResp.StatusCode(), getResp.Body))
	}
	return printThing(getResp.JSON200, thingUpdateFormat)
}
