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
	datasetUpdateName      string
	datasetUpdateFormat    string
	datasetUpdateThingUUID string
	datasetUpdateTags      []string
	datasetUpdateOut       string
)

var datasetUpdateCmd = &cobra.Command{
	Use:   "update DATASET_UUID",
	Short: "Update dataset metadata through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDatasetUpdate(args[0], cmd); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	datasetCmd.AddCommand(datasetUpdateCmd)
	datasetUpdateCmd.Flags().StringVar(&datasetUpdateName, "name", "", "New dataset name")
	datasetUpdateCmd.Flags().StringVar(&datasetUpdateFormat, "dataset-format", "", "New dataset format")
	datasetUpdateCmd.Flags().StringVar(&datasetUpdateThingUUID, "thing-uuid", "", "New thing UUID")
	datasetUpdateCmd.Flags().StringSliceVar(&datasetUpdateTags, "tags", nil, "Replace dataset tags")
	datasetUpdateCmd.Flags().StringVar(&datasetUpdateOut, "format", outputFormatTable, "Output format: table or json")
}

func runDatasetUpdate(id string, cmd *cobra.Command) error {
	if err := validateDatasetOutputFormat(datasetUpdateOut); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return err
	}
	datasetID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid dataset uuid %q", id)
	}
	body := rest.UpdateDatasetByUuidJSONRequestBody{}
	if datasetUpdateName != "" {
		body.Name = &datasetUpdateName
	}
	if datasetUpdateFormat != "" {
		format := rest.UpdateDatasetByUuidJSONBodyFormat(datasetUpdateFormat)
		body.Format = &format
	}
	if datasetUpdateThingUUID != "" {
		body.ThingUuid = &datasetUpdateThingUUID
	}
	if flag := cmd.Flags().Lookup("tags"); flag != nil && flag.Changed {
		tags := append([]string(nil), datasetUpdateTags...)
		body.Tags = &tags
	}
	if body.Name == nil && body.Format == nil && body.ThingUuid == nil && body.Tags == nil {
		return fmt.Errorf("no update fields provided")
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.UpdateDatasetByUuidWithResponse(context.Background(), rest.UuidParam(datasetID), body)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("update dataset failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	getResp, err := client.FindDatasetByUuidWithResponse(context.Background(), rest.UuidParam(datasetID))
	if err != nil {
		return err
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		return fmt.Errorf("updated dataset but failed to fetch it: %s", responseError(getResp.StatusCode(), getResp.Body))
	}
	return printDataset(getResp.JSON200, datasetUpdateOut, sizeFormatHuman)
}
