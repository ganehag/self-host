package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	userWhoamiServer string
	userWhoamiDomain string
	userWhoamiToken  string
	userWhoamiFormat string
)

var userWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the current authenticated user",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUserWhoami(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	userCmd.AddCommand(userWhoamiCmd)
	userWhoamiCmd.Flags().StringVar(&userWhoamiFormat, "format", outputFormatTable, "Output format: table or json")
}

func runUserWhoami() error {
	if err := validateDatasetOutputFormat(userWhoamiFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection(userWhoamiServer, userWhoamiDomain, userWhoamiToken)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.WhoamiWithResponse(context.Background())
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("whoami failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printUser(resp.JSON200, userWhoamiFormat)
}
