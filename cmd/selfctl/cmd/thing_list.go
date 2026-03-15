package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	thingListServer string
	thingListDomain string
	thingListToken  string
	thingListLimit  int64
	thingListOffset int64
	thingListFormat string
)

var thingListCmd = &cobra.Command{
	Use:   "list",
	Short: "List things through the Self-host API",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runThingList(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	thingCmd.AddCommand(thingListCmd)
	thingListCmd.Flags().Int64Var(&thingListLimit, "limit", 20, "Maximum number of things to list")
	thingListCmd.Flags().Int64Var(&thingListOffset, "offset", 0, "Offset into the thing list")
	thingListCmd.Flags().StringVar(&thingListFormat, "format", outputFormatTable, "Output format: table or json")
}

func runThingList() error {
	if err := validateDatasetOutputFormat(thingListFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection(thingListServer, thingListDomain, thingListToken)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	params := &rest.FindThingsParams{}
	if thingListLimit > 0 {
		v := rest.Limit(thingListLimit)
		params.Limit = &v
	}
	if thingListOffset > 0 {
		v := rest.Offset(thingListOffset)
		params.Offset = &v
	}
	resp, err := client.FindThingsWithResponse(context.Background(), params)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("list things failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printThings(*resp.JSON200, thingListFormat)
}
