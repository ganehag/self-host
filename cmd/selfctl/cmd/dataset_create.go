package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	datasetCreateServer string
	datasetCreateDomain string
	datasetCreateToken  string
	datasetCreateName   string
	datasetCreateFormat string
	datasetCreateOut    string
)

var datasetCreateCmd = &cobra.Command{
	Use:   "create NAME",
	Short: "Create a dataset through the Self-host API",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDatasetCreate(args); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	datasetCmd.AddCommand(datasetCreateCmd)
	datasetCreateCmd.Flags().StringVar(&datasetCreateName, "name", "", "Dataset name")
	datasetCreateCmd.Flags().StringVar(&datasetCreateFormat, "dataset-format", "misc", "Dataset format")
	datasetCreateCmd.Flags().StringVar(&datasetCreateOut, "format", outputFormatTable, "Output format: table or json")
}

func runDatasetCreate(args []string) error {
	if err := validateDatasetOutputFormat(datasetCreateOut); err != nil {
		return err
	}
	name := datasetCreateName
	if len(args) == 1 {
		name = args[0]
	}
	if name == "" {
		return fmt.Errorf("dataset name is required")
	}
	cfg, err := resolveAPIConnection(datasetCreateServer, datasetCreateDomain, datasetCreateToken)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	body := rest.AddDatasetsJSONRequestBody{
		Name:   name,
		Format: datasetFormatForCreate(datasetCreateFormat),
		Content: func() *[]byte {
			b := []byte{}
			return &b
		}(),
	}
	resp, err := client.AddDatasetsWithResponse(context.Background(), body)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 201 || resp.JSON201 == nil {
		return fmt.Errorf("create dataset failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printDataset(resp.JSON201, datasetCreateOut, sizeFormatHuman)
}
