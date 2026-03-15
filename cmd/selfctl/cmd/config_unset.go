package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var configUnsetCmd = &cobra.Command{
	Use:   "unset KEY",
	Short: "Unset one config value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runConfigUnset(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	configCmd.AddCommand(configUnsetCmd)
}

func runConfigUnset(key string) error {
	cfg, _, err := loadSelfctlConfig()
	if err != nil {
		return err
	}
	switch key {
	case "api.server":
		cfg.API.Server = ""
	case "api.domain":
		cfg.API.Domain = ""
	case "api.token":
		cfg.API.Token = ""
	default:
		return fmt.Errorf("unsupported config key %q", key)
	}
	path, err := saveSelfctlConfig(cfg)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "unset %s in %s\n", key, path)
	return nil
}
