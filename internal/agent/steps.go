package agent

import (
	"fmt"
	"mini-tmk-agent/internal/asr"
	"mini-tmk-agent/internal/deepseek"
)

func AudioCaptureStep() PipelineStep {
	return func(input string) string {
		return input
	}
}

func ASRStep(sourceLang string) PipelineStep {
	client := asr.NewClient()

	return func(input string) string {
		result, err := client.Transcribe(input, sourceLang)
		if err != nil {
			fmt.Println("ASR error:", err)
			return input
		}

		return result
	}
}

func TranslateStep(targetLang string, getContext func() string, addMemory func(string)) PipelineStep {
	client := deepseek.NewClient()

	return func(input string) string {
		context := getContext()

		result, err := client.Translate(
			context,
			input,
			targetLang,
		)

		if err != nil {
			fmt.Println("Translate error:", err)
			return input
		}

		addMemory(input)

		return fmt.Sprintf("%s\n%s", input, result)
	}
}

func OutputStep() PipelineStep {
	return func(input string) string {
		return input
	}
}
