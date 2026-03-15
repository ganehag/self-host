package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	thingGetServer string
	thingGetDomain string
	thingGetToken  string
	thingGetFormat string
)

var thingGetCmd = &cobra.Command{
	Use:   "get THING_UUID",
	Short: "Get one thing through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runThingGet(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	thingCmd.AddCommand(thingGetCmd)
	thingGetCmd.Flags().StringVar(&thingGetFormat, "format", outputFormatTable, "Output format: table or json")
}

func runThingGet(id string) error {
	if err := validateDatasetOutputFormat(thingGetFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection(thingGetServer, thingGetDomain, thingGetToken)
	if err != nil {
		return err
	}
	thingID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid thing uuid %q", id)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.FindThingByUuidWithResponse(context.Background(), rest.UuidParam(thingID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return fmt.Errorf("get thing failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printThing(resp.JSON200, thingGetFormat)
}
