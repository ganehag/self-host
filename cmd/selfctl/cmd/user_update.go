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
	userUpdateServer       string
	userUpdateDomain       string
	userUpdateToken        string
	userUpdateFormat       string
	userUpdateName         string
	userUpdateGroups       []string
	userUpdateGroupsAdd    []string
	userUpdateGroupsRemove []string
)

var userUpdateCmd = &cobra.Command{
	Use:   "update USER_UUID",
	Short: "Update a user through the Self-host API",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUserUpdate(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	userCmd.AddCommand(userUpdateCmd)
	userUpdateCmd.Flags().StringVar(&userUpdateFormat, "format", outputFormatTable, "Output format: table or json")
	userUpdateCmd.Flags().StringVar(&userUpdateName, "name", "", "New user name")
	userUpdateCmd.Flags().StringSliceVar(&userUpdateGroups, "groups", nil, "Replace the user's groups with this list")
	userUpdateCmd.Flags().StringSliceVar(&userUpdateGroupsAdd, "groups-add", nil, "Add the user to these groups")
	userUpdateCmd.Flags().StringSliceVar(&userUpdateGroupsRemove, "groups-remove", nil, "Remove the user from these groups")
}

func runUserUpdate(id string) error {
	if err := validateDatasetOutputFormat(userUpdateFormat); err != nil {
		return err
	}
	cfg, err := resolveAPIConnection(userUpdateServer, userUpdateDomain, userUpdateToken)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid user uuid %q", id)
	}
	if len(userUpdateGroups) > 0 && (len(userUpdateGroupsAdd) > 0 || len(userUpdateGroupsRemove) > 0) {
		return fmt.Errorf("--groups cannot be combined with --groups-add or --groups-remove")
	}

	body := rest.UpdateUserByUuidJSONRequestBody{}
	if userUpdateName != "" {
		body.Name = &userUpdateName
	}
	if len(userUpdateGroups) > 0 {
		groups := append([]string(nil), userUpdateGroups...)
		body.Groups = &groups
	}
	if len(userUpdateGroupsAdd) > 0 {
		groupsAdd := append([]string(nil), userUpdateGroupsAdd...)
		body.GroupsAdd = &groupsAdd
	}
	if len(userUpdateGroupsRemove) > 0 {
		groupsRemove := append([]string(nil), userUpdateGroupsRemove...)
		body.GroupsRemove = &groupsRemove
	}
	if body.Name == nil && body.Groups == nil && body.GroupsAdd == nil && body.GroupsRemove == nil {
		return fmt.Errorf("no update fields provided")
	}

	client, err := newAPIClient(cfg)
	if err != nil {
		return err
	}
	resp, err := client.UpdateUserByUuidWithResponse(context.Background(), rest.UuidParam(userID), body)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("update user failed: %s", responseError(resp.StatusCode(), resp.Body))
	}

	getResp, err := client.FindUserByUuidWithResponse(context.Background(), rest.UuidParam(userID))
	if err != nil {
		return err
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		return fmt.Errorf("updated user but failed to fetch it: %s", responseError(getResp.StatusCode(), getResp.Body))
	}
	return printUser(getResp.JSON200, userUpdateFormat)
}
