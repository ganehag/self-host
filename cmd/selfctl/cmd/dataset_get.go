/*
Copyright © 2021 Self-host Authors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/self-host/self-host/pkg/util/templates"
	"github.com/spf13/cobra"
)

var (
	datasetGetCmdLong = templates.LongDesc(`
		Get dataset metadata through the Self-host API.
	`)

	datasetGetCmdExample = templates.Examples(`
		# Get one dataset
		selfctl dataset get 11111111-1111-1111-1111-111111111111
	`)
)

var (
	datasetGetServer string
	datasetGetDomain string
	datasetGetToken  string
	datasetGetFormat string
	datasetGetSize   string
)

var datasetGetCmd = &cobra.Command{
	Use:     "get DATASET_UUID",
	Short:   "Get dataset metadata through the Self-host API",
	Long:    datasetGetCmdLong,
	Example: datasetGetCmdExample,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDatasetGet(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	datasetCmd.AddCommand(datasetGetCmd)
	datasetGetCmd.Flags().StringVar(&datasetGetFormat, "format", outputFormatTable, "Output format: table or json")
	datasetGetCmd.Flags().StringVar(&datasetGetSize, "size-format", sizeFormatHuman, "Size format for table output: human or bytes")
}

func runDatasetGet(id string) error {
	if err := validateDatasetOutputFormat(datasetGetFormat); err != nil {
		return err
	}
	if err := validateDatasetSizeFormat(datasetGetSize); err != nil {
		return err
	}

	cfg, err := resolveAPIConnection(datasetGetServer, datasetGetDomain, datasetGetToken)
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

	resp, err := client.FindDatasetByUuidWithResponse(context.Background(), rest.UuidParam(datasetID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("get dataset failed: %s", responseError(resp.StatusCode(), resp.Body))
	}

	return printDataset(resp.JSON200, datasetGetFormat, datasetGetSize)
}
