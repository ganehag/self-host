package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var groupDeleteCmd = &cobra.Command{
	Use:   "delete GROUP_UUID",
	Short: "Delete a group through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGroupDelete(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	groupCmd.AddCommand(groupDeleteCmd)
}

func runGroupDelete(id string) error {
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return err
	}
	groupID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid group uuid %q", id)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.DeleteGroupByUuidWithResponse(context.Background(), rest.UuidParam(groupID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("delete group failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	fmt.Fprintf(os.Stdout, "deleted group %s\n", id)
	return nil
}
