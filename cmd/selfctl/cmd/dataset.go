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
	"github.com/self-host/self-host/pkg/util/templates"
	"github.com/spf13/cobra"
)

var (
	datasetCmdLong = templates.LongDesc(`
		Interact with datasets through the Self-host API
	`)

	datasetCmdExample = templates.Examples(`
		# Upload a new dataset using config defaults from ~/.selfctl/config.yaml
		selfctl dataset upload ./big.bin --name big-upload-test

		# Upload to an existing dataset
		selfctl dataset upload ./big.bin --dataset 11111111-1111-1111-1111-111111111111
	`)
)

var datasetCmd = &cobra.Command{
	Use:     "dataset",
	Short:   "Interact with datasets through the Self-host API",
	Long:    datasetCmdLong,
	Example: datasetCmdExample,
}

func init() {
	rootCmd.AddCommand(datasetCmd)
}
