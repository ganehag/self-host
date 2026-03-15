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
	datasetDeleteServer string
	datasetDeleteDomain string
	datasetDeleteToken  string
)

var datasetDeleteCmd = &cobra.Command{
	Use:   "delete DATASET_UUID",
	Short: "Delete a dataset through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDatasetDelete(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	datasetCmd.AddCommand(datasetDeleteCmd)
}

func runDatasetDelete(id string) error {
	cfg, err := resolveAPIConnection(datasetDeleteServer, datasetDeleteDomain, datasetDeleteToken)
	if err != nil {
		return err
	}
	datasetID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid dataset uuid %q", id)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.DeleteDatasetByUuidWithResponse(context.Background(), rest.UuidParam(datasetID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("delete dataset failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	fmt.Fprintf(os.Stdout, "deleted dataset %s\n", id)
	return nil
}
