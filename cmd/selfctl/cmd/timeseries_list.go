package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	timeseriesListServer string
	timeseriesListDomain string
	timeseriesListToken  string
	timeseriesListLimit  int64
	timeseriesListOffset int64
	timeseriesListFormat string
)

var timeseriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List timeseries through the Self-host API",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runTimeseriesList(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	timeseriesCmd.AddCommand(timeseriesListCmd)
	timeseriesListCmd.Flags().Int64Var(&timeseriesListLimit, "limit", 20, "Maximum number of time series to list")
	timeseriesListCmd.Flags().Int64Var(&timeseriesListOffset, "offset", 0, "Offset into the time series list")
	timeseriesListCmd.Flags().StringVar(&timeseriesListFormat, "format", outputFormatTable, "Output format: table or json")
}

func runTimeseriesList() error {
	if err := validateDatasetOutputFormat(timeseriesListFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection(timeseriesListServer, timeseriesListDomain, timeseriesListToken)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	params := &rest.FindTimeSeriesParams{}
	if timeseriesListLimit > 0 {
		v := rest.Limit(timeseriesListLimit)
		params.Limit = &v
	}
	if timeseriesListOffset > 0 {
		v := rest.Offset(timeseriesListOffset)
		params.Offset = &v
	}
	resp, err := client.FindTimeSeriesWithResponse(context.Background(), params)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("list timeseries failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printTimeseriesList(*resp.JSON200, timeseriesListFormat)
}
