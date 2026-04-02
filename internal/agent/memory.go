package agent

import "strings"

type ContextMemory struct {
	history []string
	maxSize int
}

func NewContextMemory(maxSize int) *ContextMemory {
	return &ContextMemory{
		history: make([]string, 0),
		maxSize: maxSize,
	}
}

func (m *ContextMemory) Add(text string) {
	if text == "" {
		return
	}

	m.history = append(m.history, text)

	if len(m.history) > m.maxSize {
		m.history = m.history[1:]
	}
}

func (m *ContextMemory) GetContext() string {
	return strings.Join(m.history, "\n")
}
