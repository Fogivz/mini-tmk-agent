package agent

import "fmt"

type PipelineStep func(input string) string

type Pipeline struct {
	steps []PipelineStep
}

func NewPipeline() *Pipeline {
	return &Pipeline{
		steps: []PipelineStep{},
	}
}

func (p *Pipeline) AddStep(step PipelineStep) {
	p.steps = append(p.steps, step)
}

func (p *Pipeline) Execute(input string) string {
	result := input

	for _, step := range p.steps {
		result = step(result)
	}

	return result
}

func DebugStep(name string) PipelineStep {
	return func(input string) string {
		fmt.Println("Running step:", name)
		fmt.Println("Input:", input)
		return input
	}
}
