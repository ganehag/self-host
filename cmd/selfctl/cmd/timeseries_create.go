package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	timeseriesCreateServer     string
	timeseriesCreateDomain     string
	timeseriesCreateToken      string
	timeseriesCreateFormat     string
	timeseriesCreateName       string
	timeseriesCreateSIUnit     string
	timeseriesCreateThingUUID  string
	timeseriesCreateLowerBound float64
	timeseriesCreateUpperBound float64
	timeseriesCreateTags       []string
)

var timeseriesCreateCmd = &cobra.Command{
	Use:   "create NAME",
	Short: "Create a timeseries through the Self-host API",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runTimeseriesCreate(cmd, args); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	timeseriesCmd.AddCommand(timeseriesCreateCmd)
	timeseriesCreateCmd.Flags().StringVar(&timeseriesCreateFormat, "format", outputFormatTable, "Output format: table or json")
	timeseriesCreateCmd.Flags().StringVar(&timeseriesCreateName, "name", "", "Time series name")
	timeseriesCreateCmd.Flags().StringVar(&timeseriesCreateSIUnit, "si-unit", "", "SI unit")
	timeseriesCreateCmd.Flags().StringVar(&timeseriesCreateThingUUID, "thing-uuid", "", "Thing UUID")
	timeseriesCreateCmd.Flags().Float64Var(&timeseriesCreateLowerBound, "lower-bound", 0, "Lower value bound")
	timeseriesCreateCmd.Flags().Float64Var(&timeseriesCreateUpperBound, "upper-bound", 0, "Upper value bound")
	timeseriesCreateCmd.Flags().StringSliceVar(&timeseriesCreateTags, "tags", nil, "Time series tags")
	timeseriesCreateCmd.MarkFlagRequired("si-unit")
	timeseriesCreateCmd.Flags().Lookup("lower-bound").NoOptDefVal = "0"
	timeseriesCreateCmd.Flags().Lookup("upper-bound").NoOptDefVal = "0"
}

func runTimeseriesCreate(cmd *cobra.Command, args []string) error {
	if err := validateDatasetOutputFormat(timeseriesCreateFormat); err != nil {
		return err
	}
	name := timeseriesCreateName
	if len(args) == 1 {
		name = args[0]
	}
	if name == "" {
		return fmt.Errorf("timeseries name is required")
	}
	cfg, err := resolveAPIConnection(timeseriesCreateServer, timeseriesCreateDomain, timeseriesCreateToken)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	body := rest.AddTimeSeriesJSONRequestBody{
		Name:   name,
		SiUnit: timeseriesCreateSIUnit,
	}
	if timeseriesCreateThingUUID != "" {
		body.ThingUuid = &timeseriesCreateThingUUID
	}
	if len(timeseriesCreateTags) > 0 {
		tags := append([]string(nil), timeseriesCreateTags...)
		body.Tags = &tags
	}
	if flag := cmd.Flags().Lookup("lower-bound"); flag != nil && flag.Changed {
		body.LowerBound = &timeseriesCreateLowerBound
	}
	if flag := cmd.Flags().Lookup("upper-bound"); flag != nil && flag.Changed {
		body.UpperBound = &timeseriesCreateUpperBound
	}
	resp, err := client.AddTimeSeriesWithResponse(context.Background(), body)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 201 || resp.JSON201 == nil {
		return fmt.Errorf("create timeseries failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printTimeseries(resp.JSON201, timeseriesCreateFormat)
}
