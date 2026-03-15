package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var configSetCmd = &cobra.Command{
	Use:   "set KEY VALUE",
	Short: "Set one config value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runConfigSet(args[0], args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	configCmd.AddCommand(configSetCmd)
}

func runConfigSet(key, value string) error {
	cfg, _, err := loadSelfctlConfig()
	if err != nil {
		return err
	}
	switch key {
	case "api.server":
		cfg.API.Server = value
	case "api.domain":
		cfg.API.Domain = value
	case "api.token":
		cfg.API.Token = value
	default:
		return fmt.Errorf("unsupported config key %q", key)
	}
	path, err := saveSelfctlConfig(cfg)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "updated %s in %s\n", key, path)
	return nil
}
