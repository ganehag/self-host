package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/cobra"
)

var (
	userCreateServer string
	userCreateDomain string
	userCreateToken  string
	userCreateFormat string
)

var userCreateCmd = &cobra.Command{
	Use:   "create USER_NAME",
	Short: "Create a user through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUserCreate(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	userCmd.AddCommand(userCreateCmd)
	userCreateCmd.Flags().StringVar(&userCreateFormat, "format", outputFormatTable, "Output format: table or json")
}

func runUserCreate(name string) error {
	if err := validateDatasetOutputFormat(userCreateFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection(userCreateServer, userCreateDomain, userCreateToken)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.AddUserWithResponse(context.Background(), rest.AddUserJSONRequestBody{
		Name: name,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() != 201 || resp.JSON201 == nil {
		return fmt.Errorf("create user failed: %s", responseError(resp.StatusCode(), resp.Body))
	}
	return printUser(resp.JSON201, userCreateFormat)
}
