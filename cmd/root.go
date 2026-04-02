package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mini-tmk-agent",
	Short: "A lightweight simultaneous translation agent",
	Long:  "mini-tmk-agent is a CLI tool for real-time translation and transcript generation",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
