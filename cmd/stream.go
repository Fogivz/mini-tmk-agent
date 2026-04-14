package cmd

import (
	"go-trans/internal/agent"

	"github.com/spf13/cobra"
)

var sourceLang string
var targetLang string

var streamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Run real-time translation mode",
	Long:  "Start the agent in real-time streaming translation mode",
	Run: func(cmd *cobra.Command, args []string) {
		agent := agent.NewInterpreterAgent(
			agent.StreamMode,
			sourceLang,
			targetLang,
			"",
			"",
		)

		agent.Run()
	},
}

func init() {
	streamCmd.Flags().StringVar(
		&sourceLang,
		"source-lang",
		"zh",
		"source language",
	)

	streamCmd.Flags().StringVar(
		&targetLang,
		"target-lang",
		"en",
		"target language",
	)

	rootCmd.AddCommand(streamCmd)
}
