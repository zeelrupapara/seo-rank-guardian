package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zeelrupapara/seo-rank-guardian/app"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return app.Start()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
