package agent

import (
	"fmt"
	"mini-tmk-agent/internal/audio"
	"os"
	"strings"
)

type Mode string

const (
	StreamMode     Mode = "stream"
	TranscriptMode Mode = "transcript"
)

type InterpreterAgent struct {
	Mode           Mode
	SourceLang     string
	TargetLang     string
	InputFile      string
	OutputFile     string
	Pipeline       *Pipeline
	Memory         *ContextMemory
	OutputCallback func(string)
	stopChan       chan struct{}
}

func NewInterpreterAgent(
	mode Mode,
	sourceLang string,
	targetLang string,
	inputFile string,
	outputFile string,
) *InterpreterAgent {
	agent := &InterpreterAgent{
		Mode:       mode,
		SourceLang: sourceLang,
		TargetLang: targetLang,
		InputFile:  inputFile,
		OutputFile: outputFile,
		Memory:     NewContextMemory(5),
		stopChan:   make(chan struct{}),
	}

	agent.buildPipeline()

	return agent
}

func NewInterpreterAgentWithCallback(
	mode Mode,
	sourceLang string,
	targetLang string,
	inputFile string,
	outputFile string,
	callback func(string),
) *InterpreterAgent {
	agent := &InterpreterAgent{
		Mode:           mode,
		SourceLang:     sourceLang,
		TargetLang:     targetLang,
		InputFile:      inputFile,
		OutputFile:     outputFile,
		Memory:         NewContextMemory(5),
		OutputCallback: callback,
		stopChan:       make(chan struct{}),
	}

	agent.buildPipeline()

	return agent
}

func (a *InterpreterAgent) buildPipeline() {
	pipeline := NewPipeline()

	switch a.Mode {
	case StreamMode:
		pipeline.AddStep(AudioCaptureStep())
		pipeline.AddStep(ASRStep(a.SourceLang))
		pipeline.AddStep(TranslateStep(a.TargetLang, func() string { return a.Memory.GetContext() }, func(text string) { a.Memory.Add(text) }))
		pipeline.AddStep(OutputStep())

	case TranscriptMode:
		pipeline.AddStep(ASRStep(a.SourceLang))
		pipeline.AddStep(TranslateStep(a.TargetLang, func() string { return a.Memory.GetContext() }, func(text string) { a.Memory.Add(text) }))
		pipeline.AddStep(OutputStep())
	}

	a.Pipeline = pipeline
}

func (a *InterpreterAgent) runContinuousStream() {
	fmt.Println("实时流模式")
	fmt.Println("开始识别")

	audioChan := make(chan string, 10)

	go func() {
		index := 0

		for {
			select {
			case <-a.stopChan:
				close(audioChan)
				return
			default:
			}

			fileName := fmt.Sprintf("temp_%d.wav", index)

			err := audio.RecordWav(fileName)
			if err != nil {
				if err.Error() == "no speech detected" {
					continue
				}
				fmt.Println("record error:", err)
				continue
			}

			audioChan <- fileName
			index++
		}
	}()

	go func() {
		for audioFile := range audioChan {
			select {
			case <-a.stopChan:
				cleanupTempFile(audioFile)
				continue
			default:
			}

			result := a.Pipeline.Execute(audioFile)

			// Parse result to get original and translated
			lines := strings.Split(result, "\n")
			if len(lines) >= 2 {
				originalText := lines[0]
				translatedText := lines[1]

				output := fmt.Sprintf("原文: %s\n翻译: %s", originalText, translatedText)

				if a.OutputCallback != nil {
					a.OutputCallback(output)
				} else {
					fmt.Println("原文:", originalText)
					fmt.Println("翻译:", translatedText)
				}
			}

			cleanupTempFile(audioFile)
		}
	}()

	// Wait for stop signal to gracefully end streaming
	<-a.stopChan
}

func (a *InterpreterAgent) Run() {
	if a.stopChan == nil {
		a.stopChan = make(chan struct{})
	}

	switch a.Mode {
	case StreamMode:
		a.runContinuousStream()
	case TranscriptMode:
		_ = a.RunTranscript()
	}
}

func (a *InterpreterAgent) RunTranscript() string {
	result := a.Pipeline.Execute(a.InputFile)

	lines := strings.Split(result, "\n")
	if len(lines) >= 2 {
		originalText := lines[0]
		translatedText := lines[1]

		output := fmt.Sprintf("%s\n%s", originalText, translatedText)
		fmt.Println("转录模式")
		fmt.Println("原文:", originalText)
		fmt.Println("翻译:", translatedText)

		if a.OutputFile != "" {
			if err := os.WriteFile(a.OutputFile, []byte(output), 0644); err != nil {
				fmt.Println("Write output file failed:", err)
			} else {
				fmt.Println("Result written to", a.OutputFile)
			}
		}

		return output
	}

	return result
}

func (a *InterpreterAgent) Stop() {
	if a.stopChan == nil {
		return
	}

	select {
	case <-a.stopChan:
		// Already stopped
		return
	default:
		close(a.stopChan)
	}
}

func cleanupTempFile(filePath string) {
	if err := os.Remove(filePath); err != nil {
		fmt.Println("cleanup failed:", err)
	}
}
