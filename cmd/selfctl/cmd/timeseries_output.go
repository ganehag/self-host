package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/self-host/self-host/api/aapije/rest"
)

func printTimeseriesList(items []rest.Timeseries, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "UUID\tNAME\tUNIT\tTHING_UUID\tLOWER\tUPPER\tTAGS")
		for _, ts := range items {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				ts.Uuid,
				ts.Name,
				ts.SiUnit,
				nullableStringValue(ts.ThingUuid),
				nullableFloat(ts.LowerBound),
				nullableFloat(ts.UpperBound),
				strings.Join(ts.Tags, ", "),
			)
		}
		return w.Flush()
	}
}

func printTimeseries(ts *rest.Timeseries, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(ts)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "UUID:\t%s\n", ts.Uuid)
		fmt.Fprintf(w, "Name:\t%s\n", ts.Name)
		fmt.Fprintf(w, "SI Unit:\t%s\n", ts.SiUnit)
		fmt.Fprintf(w, "Thing UUID:\t%s\n", nullableStringValue(ts.ThingUuid))
		fmt.Fprintf(w, "Lower Bound:\t%s\n", nullableFloat(ts.LowerBound))
		fmt.Fprintf(w, "Upper Bound:\t%s\n", nullableFloat(ts.UpperBound))
		fmt.Fprintf(w, "Created By:\t%s\n", ts.CreatedBy)
		fmt.Fprintf(w, "Tags:\t%s\n", strings.Join(ts.Tags, ", "))
		return w.Flush()
	}
}

func nullableStringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func nullableFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%g", *v)
}
