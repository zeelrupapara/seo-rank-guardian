package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zeelrupapara/seo-rank-guardian/app"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start the worker process",
	RunE: func(cmd *cobra.Command, args []string) error {
		return app.StartWorker()
	},
}

func init() {
	rootCmd.AddCommand(workerCmd)
}
