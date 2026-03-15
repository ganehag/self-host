package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/self-host/self-host/api/aapije/rest"
)

func printGroups(groups []rest.Group, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(groups)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "UUID\tNAME")
		for _, group := range groups {
			fmt.Fprintf(w, "%s\t%s\n", group.Uuid, group.Name)
		}
		return w.Flush()
	}
}

func printGroup(group *rest.Group, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(group)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "UUID:\t%s\n", group.Uuid)
		fmt.Fprintf(w, "Name:\t%s\n", group.Name)
		return w.Flush()
	}
}
