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
	datasetDownloadCmdLong = templates.LongDesc(`
		Download raw dataset content through the Self-host API.
	`)

	datasetDownloadCmdExample = templates.Examples(`
		# Download one dataset to stdout
		selfctl dataset download 11111111-1111-1111-1111-111111111111

		# Download one dataset to a file
		selfctl dataset download 11111111-1111-1111-1111-111111111111 -o dataset.bin
	`)
)

var (
	datasetDownloadServer string
	datasetDownloadDomain string
	datasetDownloadToken  string
	datasetDownloadOutput string
)

var datasetDownloadCmd = &cobra.Command{
	Use:     "download DATASET_UUID",
	Short:   "Download raw dataset content through the Self-host API",
	Long:    datasetDownloadCmdLong,
	Example: datasetDownloadCmdExample,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDatasetDownload(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	datasetCmd.AddCommand(datasetDownloadCmd)
	datasetDownloadCmd.Flags().StringVarP(&datasetDownloadOutput, "output", "o", "", "Write dataset content to this file instead of stdout")
}

func runDatasetDownload(id string) error {
	cfg, err := resolveAPIConnection(datasetDownloadServer, datasetDownloadDomain, datasetDownloadToken)
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

	resp, err := client.GetRawDatasetByUuidWithResponse(context.Background(), rest.UuidParam(datasetID), &rest.GetRawDatasetByUuidParams{})
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("download dataset failed: %s", responseError(resp.StatusCode(), resp.Body))
	}

	var out *os.File
	if datasetDownloadOutput == "" {
		out = os.Stdout
	} else {
		out, err = os.Create(datasetDownloadOutput)
		if err != nil {
			return err
		}
		defer out.Close()
	}

	_, err = out.Write(resp.Body)
	return err
}
