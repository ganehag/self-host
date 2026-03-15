package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/self-host/self-host/api/aapije/rest"
)

func printThings(things []rest.Thing, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(things)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "UUID\tNAME\tSTATE\tTYPE\tTAGS")
		for _, thing := range things {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				thing.Uuid,
				thing.Name,
				thing.State,
				nullableThingType(thing.Type),
				strings.Join(thing.Tags, ", "),
			)
		}
		return w.Flush()
	}
}

func printThing(thing *rest.Thing, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(thing)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "UUID:\t%s\n", thing.Uuid)
		fmt.Fprintf(w, "Name:\t%s\n", thing.Name)
		fmt.Fprintf(w, "State:\t%s\n", thing.State)
		fmt.Fprintf(w, "Type:\t%s\n", nullableThingType(thing.Type))
		fmt.Fprintf(w, "Created By:\t%s\n", thing.CreatedBy)
		fmt.Fprintf(w, "Tags:\t%s\n", strings.Join(thing.Tags, ", "))
		return w.Flush()
	}
}

func nullableThingType(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
