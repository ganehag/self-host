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
	userTokenDeleteServer string
	userTokenDeleteDomain string
	userTokenDeleteToken  string
)

var userTokenDeleteCmd = &cobra.Command{
	Use:   "delete USER_UUID TOKEN_UUID",
	Short: "Delete a user token",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUserTokenDelete(args[0], args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	userTokenCmd.AddCommand(userTokenDeleteCmd)
}

func runUserTokenDelete(userID string, tokenID string) error {
	cfg, err := resolveAPIConnection(userTokenDeleteServer, userTokenDeleteDomain, userTokenDeleteToken)
	if err != nil {
		return err
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user uuid %q", userID)
	}
	tokenUUID, err := uuid.Parse(tokenID)
	if err != nil {
		return fmt.Errorf("invalid token uuid %q", tokenID)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.DeleteTokenForUserWithResponse(context.Background(), rest.UuidParam(userUUID), rest.TokenUUIDParam(tokenUUID))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("delete user token failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	fmt.Fprintf(os.Stdout, "deleted token %s for user %s\n", tokenID, userID)
	return nil
}
