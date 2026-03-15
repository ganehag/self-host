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

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/self-host/self-host/pkg/util/templates"
	"github.com/spf13/cobra"
)

var (
	datasetListCmdLong = templates.LongDesc(`
		List datasets through the Self-host API.
	`)

	datasetListCmdExample = templates.Examples(`
		# List datasets using config defaults
		selfctl dataset list
	`)
)

var (
	datasetListServer string
	datasetListDomain string
	datasetListToken  string
	datasetListLimit  int64
	datasetListOffset int64
	datasetListFormat string
	datasetListSize   string
)

var datasetListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List datasets through the Self-host API",
	Long:    datasetListCmdLong,
	Example: datasetListCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDatasetList(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	datasetCmd.AddCommand(datasetListCmd)
	datasetListCmd.Flags().Int64Var(&datasetListLimit, "limit", 20, "Maximum number of datasets to list")
	datasetListCmd.Flags().Int64Var(&datasetListOffset, "offset", 0, "Offset into the dataset list")
	datasetListCmd.Flags().StringVar(&datasetListFormat, "format", outputFormatTable, "Output format: table or json")
	datasetListCmd.Flags().StringVar(&datasetListSize, "size-format", sizeFormatHuman, "Size format for table output: human or bytes")
}

func runDatasetList() error {
	if err := validateDatasetOutputFormat(datasetListFormat); err != nil {
		return err
	}
	if err := validateDatasetSizeFormat(datasetListSize); err != nil {
		return err
	}

	cfg, err := resolveAPIConnection(datasetListServer, datasetListDomain, datasetListToken)
	if err != nil {
		return err
	}

	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}

	params := &rest.FindDatasetsParams{}
	if datasetListLimit > 0 {
		v := rest.Limit(datasetListLimit)
		params.Limit = &v
	}
	if datasetListOffset > 0 {
		v := rest.Offset(datasetListOffset)
		params.Offset = &v
	}

	resp, err := client.FindDatasetsWithResponse(context.Background(), params)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("list datasets failed: %s", responseError(resp.StatusCode(), resp.Body))
	}

	return printDatasetList(*resp.JSON200, datasetListFormat, datasetListSize)
}
