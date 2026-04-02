package cmd

import (
	"fmt"
	"os"

	"mini-tmk-agent/internal/agent"

	"github.com/spf13/cobra"
)

var transcriptFile string
var transcriptOutput string

var transcriptCmd = &cobra.Command{
	Use:   "transcript",
	Short: "Run an offline transcription task on a single audio file",
	Long:  "Run the ASR pipeline on one audio file and optionally write the text output to a destination file.",
	Run: func(cmd *cobra.Command, args []string) {
		if transcriptFile == "" {
			fmt.Println("Error: transcript mode requires --file <your-audio-file>")
			os.Exit(1)
		}

		agent := agent.NewInterpreterAgent(
			agent.TranscriptMode,
			"",
			"",
			transcriptFile,
			transcriptOutput,
		)

		agent.Run()
	},
}

func init() {
	transcriptCmd.Flags().StringVar(
		&transcriptFile,
		"file",
		"",
		"source audio file for transcription",
	)
	transcriptCmd.Flags().StringVar(
		&transcriptOutput,
		"output",
		"",
		"optional destination file path for transcript output",
	)

	rootCmd.AddCommand(transcriptCmd)
}
