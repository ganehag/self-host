package cmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
)

var (
	outputFormatCompletions = []string{outputFormatTable, outputFormatJSON}
	sizeFormatCompletions   = []string{sizeFormatHuman, sizeFormatBytes}
	policyEffectCompletions = []string{"allow", "deny"}
	policyActionCompletions = []string{"create", "read", "update", "delete"}
	thingStateCompletions   = []string{"active", "inactive", "passive", "archived"}
	configKeyCompletions    = []string{"api.server", "api.domain", "api.token"}
)

func init() {
	registerFlagCompletions()
	registerArgumentCompletions()
}

func registerFlagCompletions() {
	registerFixedCompletion(datasetListCmd, "format", outputFormatCompletions)
	registerFixedCompletion(datasetListCmd, "size-format", sizeFormatCompletions)
	registerFixedCompletion(datasetGetCmd, "format", outputFormatCompletions)
	registerFixedCompletion(datasetGetCmd, "size-format", sizeFormatCompletions)
	registerFixedCompletion(datasetCreateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(datasetUpdateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(datasetUploadCmd, "format", nil)

	registerFixedCompletion(userListCmd, "format", outputFormatCompletions)
	registerFixedCompletion(userGetCmd, "format", outputFormatCompletions)
	registerFixedCompletion(userWhoamiCmd, "format", outputFormatCompletions)
	registerFixedCompletion(userCreateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(userUpdateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(userTokenListCmd, "format", outputFormatCompletions)
	registerFixedCompletion(userTokenCreateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(userPolicyListCmd, "format", outputFormatCompletions)

	registerFixedCompletion(groupListCmd, "format", outputFormatCompletions)
	registerFixedCompletion(groupGetCmd, "format", outputFormatCompletions)
	registerFixedCompletion(groupCreateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(groupUpdateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(groupPolicyListCmd, "format", outputFormatCompletions)

	registerFixedCompletion(policyListCmd, "format", outputFormatCompletions)
	registerFixedCompletion(policyGetCmd, "format", outputFormatCompletions)
	registerFixedCompletion(policyCreateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(policyUpdateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(policyExplainCmd, "format", outputFormatCompletions)
	registerFixedCompletion(policyCreateCmd, "effect", policyEffectCompletions)
	registerFixedCompletion(policyUpdateCmd, "effect", policyEffectCompletions)
	registerFixedCompletion(policyCreateCmd, "action", policyActionCompletions)
	registerFixedCompletion(policyUpdateCmd, "action", policyActionCompletions)

	registerFixedCompletion(thingListCmd, "format", outputFormatCompletions)
	registerFixedCompletion(thingGetCmd, "format", outputFormatCompletions)
	registerFixedCompletion(thingCreateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(thingUpdateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(thingUpdateCmd, "state", thingStateCompletions)
	registerFixedCompletion(thingDatasetsCmd, "format", outputFormatCompletions)
	registerFixedCompletion(thingDatasetsCmd, "size-format", sizeFormatCompletions)
	registerFixedCompletion(thingTimeseriesCmd, "format", outputFormatCompletions)

	registerFixedCompletion(timeseriesListCmd, "format", outputFormatCompletions)
	registerFixedCompletion(timeseriesGetCmd, "format", outputFormatCompletions)
	registerFixedCompletion(timeseriesCreateCmd, "format", outputFormatCompletions)
	registerFixedCompletion(timeseriesUpdateCmd, "format", outputFormatCompletions)

	registerFixedCompletion(configViewCmd, "format", outputFormatCompletions)
}

func registerArgumentCompletions() {
	configSetCmd.ValidArgsFunction = completeConfigKeys
	configUnsetCmd.ValidArgsFunction = completeConfigKeys

	thingGetCmd.ValidArgsFunction = completeThingUUIDs
	thingUpdateCmd.ValidArgsFunction = completeThingUUIDs
	thingDeleteCmd.ValidArgsFunction = completeThingUUIDs
	thingDatasetsCmd.ValidArgsFunction = completeThingUUIDs
	thingTimeseriesCmd.ValidArgsFunction = completeThingUUIDs

	datasetGetCmd.ValidArgsFunction = completeDatasetUUIDs
	datasetUpdateCmd.ValidArgsFunction = completeDatasetUUIDs
	datasetDeleteCmd.ValidArgsFunction = completeDatasetUUIDs
	datasetDownloadCmd.ValidArgsFunction = completeDatasetUUIDs

	timeseriesGetCmd.ValidArgsFunction = completeTimeseriesUUIDs
	timeseriesUpdateCmd.ValidArgsFunction = completeTimeseriesUUIDs
	timeseriesDeleteCmd.ValidArgsFunction = completeTimeseriesUUIDs

	userGetCmd.ValidArgsFunction = completeUserUUIDs
	userDeleteCmd.ValidArgsFunction = completeUserUUIDs
	userUpdateCmd.ValidArgsFunction = completeUserUUIDs
	userTokenListCmd.ValidArgsFunction = completeUserUUIDs
	userTokenCreateCmd.ValidArgsFunction = completeUserUUIDs
	userPolicyListCmd.ValidArgsFunction = completeUserUUIDs

	groupGetCmd.ValidArgsFunction = completeGroupUUIDs
	groupDeleteCmd.ValidArgsFunction = completeGroupUUIDs
	groupUpdateCmd.ValidArgsFunction = completeGroupUUIDs
	groupPolicyListCmd.ValidArgsFunction = completeGroupUUIDs

	policyGetCmd.ValidArgsFunction = completePolicyUUIDs
	policyDeleteCmd.ValidArgsFunction = completePolicyUUIDs
	policyUpdateCmd.ValidArgsFunction = completePolicyUUIDs
}

func registerFixedCompletion(cmd *cobra.Command, flag string, values []string) {
	if cmd == nil {
		return
	}
	_ = cmd.RegisterFlagCompletionFunc(flag, func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if values == nil {
			return nil, cobra.ShellCompDirectiveDefault
		}
		return filterPrefix(values, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
}

func completeConfigKeys(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return filterPrefix(configKeyCompletions, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeDatasetUUIDs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	resp, err := client.FindDatasetsWithResponse(context.Background(), nil)
	if err != nil || resp.StatusCode() != 200 || resp.JSON200 == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	candidates := make([]string, 0, len(*resp.JSON200))
	for _, item := range *resp.JSON200 {
		candidates = append(candidates, item.Uuid+"\t"+item.Name)
	}
	return filterPrefix(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeThingUUIDs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	resp, err := client.FindThingsWithResponse(context.Background(), nil)
	if err != nil || resp.StatusCode() != 200 || resp.JSON200 == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	candidates := make([]string, 0, len(*resp.JSON200))
	for _, item := range *resp.JSON200 {
		candidates = append(candidates, item.Uuid+"\t"+item.Name)
	}
	return filterPrefix(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeTimeseriesUUIDs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	resp, err := client.FindTimeSeriesWithResponse(context.Background(), nil)
	if err != nil || resp.StatusCode() != 200 || resp.JSON200 == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	candidates := make([]string, 0, len(*resp.JSON200))
	for _, item := range *resp.JSON200 {
		candidates = append(candidates, item.Uuid+"\t"+item.Name)
	}
	return filterPrefix(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeUserUUIDs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	resp, err := client.FindUsersWithResponse(context.Background(), nil)
	if err != nil || resp.StatusCode() != 200 || resp.JSON200 == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	candidates := make([]string, 0, len(*resp.JSON200))
	for _, item := range *resp.JSON200 {
		candidates = append(candidates, item.Uuid+"\t"+item.Name)
	}
	return filterPrefix(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeGroupUUIDs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	resp, err := client.FindGroupsWithResponse(context.Background(), nil)
	if err != nil || resp.StatusCode() != 200 || resp.JSON200 == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	candidates := make([]string, 0, len(*resp.JSON200))
	for _, item := range *resp.JSON200 {
		candidates = append(candidates, item.Uuid+"\t"+item.Name)
	}
	return filterPrefix(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completePolicyUUIDs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := resolveAPIConnection("", "", "")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	resp, err := client.FindPoliciesWithResponse(context.Background(), nil)
	if err != nil || resp.StatusCode() != 200 || resp.JSON200 == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	candidates := make([]string, 0, len(*resp.JSON200))
	for _, item := range *resp.JSON200 {
		candidates = append(candidates, item.Uuid+"\t"+string(item.Effect)+" "+string(item.Action)+" "+item.Resource)
	}
	return filterPrefix(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func filterPrefix(items []string, prefix string) []string {
	if prefix == "" {
		return items
	}
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
