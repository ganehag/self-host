package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/self-host/self-host/api/aapije/rest"
)

func printPolicies(items []rest.Policy, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "UUID\tGROUP_UUID\tPRIORITY\tEFFECT\tACTION\tRESOURCE")
		for _, item := range items {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n", item.Uuid, item.GroupUuid, item.Priority, item.Effect, item.Action, item.Resource)
		}
		return w.Flush()
	}
}

func printPolicy(item *rest.Policy, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(item)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "UUID:\t%s\n", item.Uuid)
		fmt.Fprintf(w, "Group UUID:\t%s\n", item.GroupUuid)
		fmt.Fprintf(w, "Priority:\t%d\n", item.Priority)
		fmt.Fprintf(w, "Effect:\t%s\n", item.Effect)
		fmt.Fprintf(w, "Action:\t%s\n", item.Action)
		fmt.Fprintf(w, "Resource:\t%s\n", item.Resource)
		return w.Flush()
	}
}

func printPolicyDecision(item *rest.AuthorizationDecision, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(item)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Access:\t%t\n", item.Access)
		fmt.Fprintf(w, "Action:\t%s\n", item.Action)
		fmt.Fprintf(w, "Resource:\t%s\n", item.Resource)
		fmt.Fprintf(w, "Effect:\t%s\n", nullableEffect(item.Effect))
		fmt.Fprintf(w, "Group UUID:\t%s\n", nullableStringValue(item.GroupUuid))
		fmt.Fprintf(w, "Policy UUID:\t%s\n", nullableStringValue(item.PolicyUuid))
		fmt.Fprintf(w, "Priority:\t%s\n", nullableInt32(item.Priority))
		fmt.Fprintf(w, "Matched Pattern:\t%s\n", nullableStringValue(item.MatchedPattern))
		return w.Flush()
	}
}

func nullableInt32(v *int32) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%d", *v)
}

func nullableEffect(v *rest.AuthorizationDecisionEffect) string {
	if v == nil {
		return ""
	}
	return string(*v)
}
