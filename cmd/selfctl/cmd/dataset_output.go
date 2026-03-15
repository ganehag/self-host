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
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/self-host/self-host/api/aapije/rest"
)

const (
	outputFormatTable = "table"
	outputFormatJSON  = "json"
	sizeFormatHuman   = "human"
	sizeFormatBytes   = "bytes"
)

func validateDatasetOutputFormat(format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatTable, outputFormatJSON:
		return nil
	default:
		return fmt.Errorf("unsupported output format %q; supported values: table, json", format)
	}
}

func validateDatasetSizeFormat(format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case sizeFormatHuman, sizeFormatBytes:
		return nil
	default:
		return fmt.Errorf("unsupported size format %q; supported values: human, bytes", format)
	}
}

func printDatasetList(datasets []rest.Dataset, format string, sizeFormat string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(datasets)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "UUID\tNAME\tFORMAT\tSIZE\tUPDATED")
		for _, ds := range datasets {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				ds.Uuid,
				ds.Name,
				ds.Format,
				formatDatasetSize(ds.Size, sizeFormat),
				ds.Updated.Format("2006-01-02 15:04:05"),
			)
		}
		return w.Flush()
	}
}

func printDataset(ds *rest.Dataset, format string, sizeFormat string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(ds)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "UUID:\t%s\n", ds.Uuid)
		fmt.Fprintf(w, "Name:\t%s\n", ds.Name)
		fmt.Fprintf(w, "Format:\t%s\n", ds.Format)
		fmt.Fprintf(w, "Size:\t%s\n", formatDatasetSize(ds.Size, sizeFormat))
		fmt.Fprintf(w, "Checksum:\t%s\n", ds.Checksum)
		fmt.Fprintf(w, "Created:\t%s\n", ds.Created.Format("2006-01-02 15:04:05 -0700"))
		fmt.Fprintf(w, "Updated:\t%s\n", ds.Updated.Format("2006-01-02 15:04:05 -0700"))
		fmt.Fprintf(w, "Created By:\t%s\n", ds.CreatedBy)
		fmt.Fprintf(w, "Updated By:\t%s\n", ds.UpdatedBy)
		if ds.ThingUuid != nil && *ds.ThingUuid != "" {
			fmt.Fprintf(w, "Thing UUID:\t%s\n", *ds.ThingUuid)
		}
		if len(ds.Tags) > 0 {
			fmt.Fprintf(w, "Tags:\t%s\n", strings.Join(ds.Tags, ", "))
		} else {
			fmt.Fprintf(w, "Tags:\t\n")
		}
		return w.Flush()
	}
}

func formatDatasetSize(size int64, sizeFormat string) string {
	if strings.EqualFold(strings.TrimSpace(sizeFormat), sizeFormatBytes) {
		return fmt.Sprintf("%d", size)
	}
	return humanBytes(size)
}

func humanBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}

	units := []string{"kB", "MB", "GB", "TB", "PB"}
	value := float64(size)
	unit := "B"
	for _, candidate := range units {
		value /= 1024
		unit = candidate
		if value < 1024 {
			break
		}
	}

	if value >= 10 || math.Mod(value, 1) == 0 {
		return fmt.Sprintf("%.0f %s", value, unit)
	}

	return fmt.Sprintf("%.1f %s", value, unit)
}
