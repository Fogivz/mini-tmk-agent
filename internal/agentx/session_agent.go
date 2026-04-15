package agentx

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go-trans/internal/deepseek"
)

type Turn struct {
	SpeakerID      string `json:"speaker_id"`
	SourceLang     string `json:"source_lang"`
	TargetLang     string `json:"target_lang"`
	OriginalText   string `json:"original_text"`
	TranslatedText string `json:"translated_text"`
	TimestampMs    int64  `json:"ts"`
}

type Report struct {
	SessionID   string    `json:"session_id"`
	GeneratedAt time.Time `json:"generated_at"`
	Summary     string    `json:"summary"`
	Viewpoints  string    `json:"viewpoints"`
	Turns       []Turn    `json:"turns"`
}

type Options struct {
	SessionID    string
	KnowledgeDir string
	ReportDir    string
	MaxRagDocs   int
	MCPEndpoint  string
}

type SessionAgent struct {
	mu      sync.Mutex
	opts    Options
	llm     *deepseek.Client
	rag     *Retriever
	mcp     MCPContextProvider
	skills  map[string]Skill
	turns   []Turn
	started time.Time
}

func NewSessionAgent(opts Options, llm *deepseek.Client) *SessionAgent {
	if opts.MaxRagDocs <= 0 {
		opts.MaxRagDocs = 3
	}
	if strings.TrimSpace(opts.ReportDir) == "" {
		opts.ReportDir = "reports"
	}
	if llm == nil {
		llm = deepseek.NewClient()
	}

	mcpProvider, err := NewMCPProviderFromEndpoint(opts.MCPEndpoint)
	if err != nil {
		log.Printf("agentx mcp init failed, fallback to noop: %v", err)
		mcpProvider = NoopMCPProvider{}
	}

	a := &SessionAgent{
		opts:    opts,
		llm:     llm,
		rag:     NewRetriever(opts.KnowledgeDir),
		mcp:     mcpProvider,
		skills:  make(map[string]Skill),
		turns:   make([]Turn, 0, 64),
		started: time.Now(),
	}
	a.RegisterSkill(SummarySkill{})
	a.RegisterSkill(ViewpointsSkill{})
	return a
}

func (a *SessionAgent) Close() error {
	if a == nil || a.mcp == nil {
		return nil
	}
	return a.mcp.Close()
}

func (a *SessionAgent) RegisterSkill(skill Skill) {
	if skill == nil {
		return
	}
	a.skills[skill.Name()] = skill
}

func (a *SessionAgent) AddTurn(turn Turn) {
	if strings.TrimSpace(turn.OriginalText) == "" {
		return
	}
	if turn.TimestampMs == 0 {
		turn.TimestampMs = time.Now().UnixMilli()
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.turns = append(a.turns, turn)
}

func (a *SessionAgent) GenerateReport(ctx context.Context) (Report, error) {
	a.mu.Lock()
	turns := make([]Turn, len(a.turns))
	copy(turns, a.turns)
	a.mu.Unlock()

	if len(turns) == 0 {
		return Report{}, fmt.Errorf("no turns captured")
	}

	sort.Slice(turns, func(i, j int) bool {
		return turns[i].TimestampMs < turns[j].TimestampMs
	})

	summary, err := a.runSkill(ctx, SkillSummary)
	if err != nil {
		return Report{}, fmt.Errorf("run summary skill failed: %w", err)
	}
	viewpoints, err := a.runSkill(ctx, SkillViewpoints)
	if err != nil {
		return Report{}, fmt.Errorf("run viewpoints skill failed: %w", err)
	}

	report := Report{
		SessionID:   a.opts.SessionID,
		GeneratedAt: time.Now(),
		Summary:     strings.TrimSpace(summary),
		Viewpoints:  strings.TrimSpace(viewpoints),
		Turns:       turns,
	}
	return report, nil
}

func (a *SessionAgent) SaveTurns() (string, error) {
	a.mu.Lock()
	turns := make([]Turn, len(a.turns))
	copy(turns, a.turns)
	a.mu.Unlock()

	if len(turns) == 0 {
		return "", nil
	}

	file, err := os.CreateTemp("", "go_trans_turns_*.json")
	if err != nil {
		return "", fmt.Errorf("create temp turns file failed: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close temp turns file failed: %w", err)
	}

	bs, err := json.MarshalIndent(turns, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal turns failed: %w", err)
	}
	if err := os.WriteFile(path, bs, 0o644); err != nil {
		return "", fmt.Errorf("write turns failed: %w", err)
	}
	return path, nil
}

func (a *SessionAgent) SaveReport(report Report) (string, error) {
	if strings.TrimSpace(a.opts.ReportDir) == "" {
		return "", fmt.Errorf("empty report dir")
	}
	if err := os.MkdirAll(a.opts.ReportDir, 0o755); err != nil {
		return "", fmt.Errorf("create report dir failed: %w", err)
	}

	fileName := fmt.Sprintf("session_%s_%d.json", sanitize(report.SessionID), time.Now().Unix())
	path := filepath.Join(a.opts.ReportDir, fileName)

	bs, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report failed: %w", err)
	}
	if err := os.WriteFile(path, bs, 0o644); err != nil {
		return "", fmt.Errorf("write report failed: %w", err)
	}
	return path, nil
}

func (a *SessionAgent) RetrieveRAG(query string) string {
	if a.rag == nil {
		return ""
	}
	snippets := a.rag.Retrieve(query, a.opts.MaxRagDocs)
	if len(snippets) == 0 {
		return ""
	}
	return formatRag(snippets)
}

func (a *SessionAgent) buildSharedPrompt(ctx context.Context, task string) string {
	a.mu.Lock()
	turns := make([]Turn, len(a.turns))
	copy(turns, a.turns)
	a.mu.Unlock()

	conversation := turnsToText(turns)
	snippets := a.rag.Retrieve(conversation, a.opts.MaxRagDocs)
	ragText := formatRag(snippets)

	mcpCtx, err := a.mcp.FetchContext(ctx, conversation)
	if err != nil {
		log.Printf("agentx mcp fetch failed: %v", err)
	}
	if strings.TrimSpace(mcpCtx) == "" {
		mcpCtx = "(none)"
	}

	return fmt.Sprintf("任务:\n%s\n\n会话记录:\n%s\n\nRAG知识库检索:\n%s\n\nMCP上下文:\n%s", task, conversation, ragText, mcpCtx)
}

func turnsToText(turns []Turn) string {
	if len(turns) == 0 {
		return ""
	}
	lines := make([]string, 0, len(turns))
	for _, t := range turns {
		lines = append(lines, fmt.Sprintf("[%s] 原文:%s | 译文:%s", t.SpeakerID, t.OriginalText, t.TranslatedText))
	}
	return strings.Join(lines, "\n")
}

func sanitize(s string) string {
	cleaned := strings.TrimSpace(s)
	if cleaned == "" {
		return "session"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	return replacer.Replace(cleaned)
}
