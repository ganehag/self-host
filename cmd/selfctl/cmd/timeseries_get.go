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
	timeseriesGetServer string
	timeseriesGetDomain string
	timeseriesGetToken  string
	timeseriesGetFormat string
)

var timeseriesGetCmd = &cobra.Command{
	Use:   "get TIMESERIES_UUID",
	Short: "Get one timeseries through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runTimeseriesGet(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	timeseriesCmd.AddCommand(timeseriesGetCmd)
	timeseriesGetCmd.Flags().StringVar(&timeseriesGetFormat, "format", outputFormatTable, "Output format: table or json")
}

func runTimeseriesGet(id string) error {
	if err := validateDatasetOutputFormat(timeseriesGetFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection(timeseriesGetServer, timeseriesGetDomain, timeseriesGetToken)
	if err != nil {
		return err
	}
	tsID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid timeseries uuid %q", id)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.FindTimeSeriesByUuidWithResponse(context.Background(), rest.UuidParam(tsID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("get timeseries failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printTimeseries(resp.JSON200, timeseriesGetFormat)
}
