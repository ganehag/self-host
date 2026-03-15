package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var thingTimeseriesFormat string

var thingTimeseriesCmd = &cobra.Command{
	Use:   "timeseries THING_UUID",
	Short: "List timeseries attached to a thing",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runThingTimeseries(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	thingCmd.AddCommand(thingTimeseriesCmd)
	thingTimeseriesCmd.Flags().StringVar(&thingTimeseriesFormat, "format", outputFormatTable, "Output format: table or json")
}

func runThingTimeseries(id string) error {
	if err := validateDatasetOutputFormat(thingTimeseriesFormat); err != nil {
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
	resp, err := client.FindTimeSeriesForThingWithResponse(context.Background(), rest.UuidParam(thingID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("list thing timeseries failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printTimeseriesList(*resp.JSON200, thingTimeseriesFormat)
}
