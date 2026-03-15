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
	thingDatasetsFormat     string
	thingDatasetsSizeFormat string
)

var thingDatasetsCmd = &cobra.Command{
	Use:   "datasets THING_UUID",
	Short: "List datasets attached to a thing",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runThingDatasets(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	thingCmd.AddCommand(thingDatasetsCmd)
	thingDatasetsCmd.Flags().StringVar(&thingDatasetsFormat, "format", outputFormatTable, "Output format: table or json")
	thingDatasetsCmd.Flags().StringVar(&thingDatasetsSizeFormat, "size-format", sizeFormatHuman, "Dataset size display: human or bytes")
}

func runThingDatasets(id string) error {
	if err := validateDatasetOutputFormat(thingDatasetsFormat); err != nil {
		return err
	}
	if err := validateDatasetSizeFormat(thingDatasetsSizeFormat); err != nil {
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
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.FindDatasetsForThingWithResponse(context.Background(), rest.UuidParam(thingID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("list thing datasets failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printDatasetList(*resp.JSON200, thingDatasetsFormat, thingDatasetsSizeFormat)
}
