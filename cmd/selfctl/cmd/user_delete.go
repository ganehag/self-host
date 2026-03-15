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
	userDeleteServer string
	userDeleteDomain string
	userDeleteToken  string
)

var userDeleteCmd = &cobra.Command{
	Use:   "delete USER_UUID",
	Short: "Delete a user through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUserDelete(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	userCmd.AddCommand(userDeleteCmd)
}

func runUserDelete(id string) error {
	cfg, err := resolveAPIConnection(userDeleteServer, userDeleteDomain, userDeleteToken)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid user uuid %q", id)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.DeleteUserByUuidWithResponse(context.Background(), rest.UuidParam(userID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("delete user failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	fmt.Fprintf(os.Stdout, "deleted user %s\n", id)
	return nil
}
