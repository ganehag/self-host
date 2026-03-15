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
	timeseriesUpdateName       string
	timeseriesUpdateSIUnit     string
	timeseriesUpdateThingUUID  string
	timeseriesUpdateLowerBound float64
	timeseriesUpdateUpperBound float64
	timeseriesUpdateTags       []string
	timeseriesUpdateFormat     string
)

var timeseriesUpdateCmd = &cobra.Command{
	Use:   "update TIMESERIES_UUID",
	Short: "Update a timeseries through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runTimeseriesUpdate(args[0], cmd); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	timeseriesCmd.AddCommand(timeseriesUpdateCmd)
	timeseriesUpdateCmd.Flags().StringVar(&timeseriesUpdateName, "name", "", "New timeseries name")
	timeseriesUpdateCmd.Flags().StringVar(&timeseriesUpdateSIUnit, "si-unit", "", "New SI unit")
	timeseriesUpdateCmd.Flags().StringVar(&timeseriesUpdateThingUUID, "thing-uuid", "", "New thing UUID")
	timeseriesUpdateCmd.Flags().Float64Var(&timeseriesUpdateLowerBound, "lower-bound", 0, "New lower value bound")
	timeseriesUpdateCmd.Flags().Float64Var(&timeseriesUpdateUpperBound, "upper-bound", 0, "New upper value bound")
	timeseriesUpdateCmd.Flags().StringSliceVar(&timeseriesUpdateTags, "tags", nil, "Replace timeseries tags")
	timeseriesUpdateCmd.Flags().StringVar(&timeseriesUpdateFormat, "format", outputFormatTable, "Output format: table or json")
}

func runTimeseriesUpdate(id string, cmd *cobra.Command) error {
	if err := validateDatasetOutputFormat(timeseriesUpdateFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return err
	}
	tsID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid timeseries uuid %q", id)
	}
	body := rest.UpdateTimeseriesByUuidJSONRequestBody{}
	if timeseriesUpdateName != "" {
		body.Name = &timeseriesUpdateName
	}
	if timeseriesUpdateSIUnit != "" {
		body.SiUnit = &timeseriesUpdateSIUnit
	}
	if timeseriesUpdateThingUUID != "" {
		body.ThingUuid = &timeseriesUpdateThingUUID
	}
	if flag := cmd.Flags().Lookup("lower-bound"); flag != nil && flag.Changed {
		body.LowerBound = &timeseriesUpdateLowerBound
	}
	if flag := cmd.Flags().Lookup("upper-bound"); flag != nil && flag.Changed {
		body.UpperBound = &timeseriesUpdateUpperBound
	}
	if flag := cmd.Flags().Lookup("tags"); flag != nil && flag.Changed {
		tags := append([]string(nil), timeseriesUpdateTags...)
		body.Tags = &tags
	}
	if body.Name == nil && body.SiUnit == nil && body.ThingUuid == nil && body.LowerBound == nil && body.UpperBound == nil && body.Tags == nil {
		return fmt.Errorf("no update fields provided")
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.UpdateTimeseriesByUuidWithResponse(context.Background(), rest.UuidParam(tsID), body)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("update timeseries failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	getResp, err := client.FindTimeSeriesByUuidWithResponse(context.Background(), rest.UuidParam(tsID))
	if err != nil {
		return err
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		return fmt.Errorf("updated timeseries but failed to fetch it: %s", responseError(getResp.StatusCode(), getResp.Body))
	}
	return printTimeseries(getResp.JSON200, timeseriesUpdateFormat)
}
