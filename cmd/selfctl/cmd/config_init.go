package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configInitServer string
	configInitDomain string
	configInitToken  string
)

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create or overwrite the selfctl config file",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runConfigInit(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configInitCmd.Flags().StringVar(&configInitServer, "server", apiServer, "Default API base URL")
	configInitCmd.Flags().StringVar(&configInitDomain, "domain", apiDomain, "Default API domain")
	configInitCmd.Flags().StringVar(&configInitToken, "token", apiToken, "Default API token")
}

func runConfigInit() error {
	cfg := selfctlConfig{
		API: apiConfigSection{
			Server: configInitServer,
			Domain: configInitDomain,
			Token:  configInitToken,
		},
	}
	path, err := saveSelfctlConfig(cfg)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "wrote config %s\n", path)
	return nil
}
