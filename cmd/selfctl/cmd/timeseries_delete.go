package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var timeseriesDeleteCmd = &cobra.Command{
	Use:   "delete TIMESERIES_UUID",
	Short: "Delete a timeseries through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runTimeseriesDelete(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	timeseriesCmd.AddCommand(timeseriesDeleteCmd)
}

func runTimeseriesDelete(id string) error {
	cfg, err := resolveAPIConnection("", "", "")
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
	resp, err := client.DeleteTimeSeriesByUuidWithResponse(context.Background(), rest.UuidParam(tsID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("delete timeseries failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	fmt.Fprintf(os.Stdout, "deleted timeseries %s\n", id)
	return nil
}
