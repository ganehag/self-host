package cmd

import (
	"github.com/self-host/self-host/pkg/util/templates"
	"github.com/spf13/cobra"
)

var (
	timeseriesCmdLong = templates.LongDesc(`
		Interact with timeseries through the Self-host API
	`)
)

var timeseriesCmd = &cobra.Command{
	Use:   "timeseries",
	Short: "Interact with timeseries through the Self-host API",
	Long:  timeseriesCmdLong,
}

func init() {
	rootCmd.AddCommand(timeseriesCmd)
}
