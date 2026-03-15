package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var thingDeleteCmd = &cobra.Command{
	Use:   "delete THING_UUID",
	Short: "Delete a thing through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runThingDelete(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	thingCmd.AddCommand(thingDeleteCmd)
}

func runThingDelete(id string) error {
	cfg, err := resolveAPIConnection("", "", "")
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
	resp, err := client.DeleteThingByUuidWithResponse(context.Background(), rest.UuidParam(thingID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("delete thing failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	fmt.Fprintf(os.Stdout, "deleted thing %s\n", id)
	return nil
}
