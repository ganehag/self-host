package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var configViewFormat string

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Show the current selfctl config file",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runConfigView(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	configCmd.AddCommand(configViewCmd)
	configViewCmd.Flags().StringVar(&configViewFormat, "format", outputFormatTable, "Output format: table or json")
}

func runConfigView() error {
	if err := validateDatasetOutputFormat(configViewFormat); err != nil {
		return err
	}
	cfg, path, err := loadSelfctlConfig()
	if err != nil {
		return err
	}
	if configViewFormat == outputFormatJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Path string           `json:"path"`
			API  apiConfigSection `json:"api"`
		}{Path: path, API: redactAPIConfig(cfg.API)})
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Path:\t%s\n", path)
	fmt.Fprintf(w, "API Server:\t%s\n", cfg.API.Server)
	fmt.Fprintf(w, "API Domain:\t%s\n", cfg.API.Domain)
	fmt.Fprintf(w, "API Token:\t%s\n", redactToken(cfg.API.Token))
	return w.Flush()
}

func redactAPIConfig(cfg apiConfigSection) apiConfigSection {
	cfg.Token = redactToken(cfg.Token)
	return cfg
}

func redactToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 4 {
		return "****"
	}
	return token[:2] + strings.Repeat("*", len(token)-4) + token[len(token)-2:]
}
