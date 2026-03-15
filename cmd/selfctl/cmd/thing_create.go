package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	thingCreateServer string
	thingCreateDomain string
	thingCreateToken  string
	thingCreateFormat string
	thingCreateName   string
	thingCreateType   string
	thingCreateTags   []string
)

var thingCreateCmd = &cobra.Command{
	Use:   "create NAME",
	Short: "Create a thing through the Self-host API",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runThingCreate(args); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	thingCmd.AddCommand(thingCreateCmd)
	thingCreateCmd.Flags().StringVar(&thingCreateFormat, "format", outputFormatTable, "Output format: table or json")
	thingCreateCmd.Flags().StringVar(&thingCreateName, "name", "", "Thing name")
	thingCreateCmd.Flags().StringVar(&thingCreateType, "thing-type", "", "Thing type")
	thingCreateCmd.Flags().StringSliceVar(&thingCreateTags, "tags", nil, "Thing tags")
}

func runThingCreate(args []string) error {
	if err := validateDatasetOutputFormat(thingCreateFormat); err != nil {
		return err
	}
	name := thingCreateName
	if len(args) == 1 {
		name = args[0]
	}
	if name == "" {
		return fmt.Errorf("thing name is required")
	}
	cfg, err := resolveAPIConnection(thingCreateServer, thingCreateDomain, thingCreateToken)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	body := rest.AddThingJSONRequestBody{
		Name: name,
	}
	if thingCreateType != "" {
		body.Type = &thingCreateType
	}
	if len(thingCreateTags) > 0 {
		tags := append([]string(nil), thingCreateTags...)
		body.Tags = &tags
	}
	resp, err := client.AddThingWithResponse(context.Background(), body)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 201 || resp.JSON201 == nil {
		return fmt.Errorf("create thing failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printThing(resp.JSON201, thingCreateFormat)
}
