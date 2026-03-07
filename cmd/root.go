package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "srg",
	Short: "SEO Rank Guardian",
	Long:  "SEO Rank Guardian - SaaS platform for SEO rank tracking and analysis",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
