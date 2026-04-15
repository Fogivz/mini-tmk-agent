package agentx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type MCPContextProvider interface {
	FetchContext(ctx context.Context, query string) (string, error)
	Close() error
}

type NoopMCPProvider struct{}

func (NoopMCPProvider) FetchContext(ctx context.Context, query string) (string, error) {
	return "", nil
}

func (NoopMCPProvider) Close() error {
	return nil
}

type HTTPMCPProvider struct {
	endpoint string
	client   *http.Client
}

func NewHTTPMCPProvider(endpoint string) (*HTTPMCPProvider, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("empty endpoint")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid mcp endpoint: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported mcp endpoint scheme: %s", u.Scheme)
	}
	return &HTTPMCPProvider{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
	}, nil
}

func (p *HTTPMCPProvider) FetchContext(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", nil
	}

	u, err := url.Parse(p.endpoint)
	if err != nil {
		return "", err
	}
	qs := u.Query()
	qs.Set("query", query)
	u.RawQuery = qs.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("mcp http provider returned %d", resp.StatusCode)
	}

	text := extractTextFromBody(body)
	return strings.TrimSpace(text), nil
}

func (p *HTTPMCPProvider) Close() error {
	return nil
}

type StdioMCPProvider struct {
	cli *client.Client
}

// NewStdioMCPProvider 启动一个本地 MCP Server 进程
func NewStdioMCPProvider(command string, args []string) (*StdioMCPProvider, error) {
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("empty command")
	}

	cli, err := client.NewStdioMCPClient(command, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("start mcp client failed: %w", err)
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "go-trans-agent",
		Version: "1.0.0",
	}

	_, err = cli.Initialize(context.Background(), initRequest)
	if err != nil {
		cli.Close()
		return nil, fmt.Errorf("initialize mcp client failed: %w", err)
	}

	return &StdioMCPProvider{
		cli: cli,
	}, nil
}

func (p *StdioMCPProvider) FetchContext(ctx context.Context, query string) (string, error) {
	if p.cli == nil || strings.TrimSpace(query) == "" {
		return "", nil
	}

	tools, err := p.cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return "", fmt.Errorf("list mcp tools failed: %w", err)
	}
	if len(tools.Tools) == 0 {
		return "", nil
	}

	ranked := rankTools(tools.Tools)
	var errs []string
	for _, tool := range ranked {
		res, callErr := p.callToolWithQuery(ctx, tool, query)
		if callErr != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", tool.Name, callErr))
			continue
		}
		if strings.TrimSpace(res) != "" {
			return res, nil
		}
	}

	if len(errs) > 0 {
		return "", fmt.Errorf("all mcp tools failed: %s", strings.Join(errs, " | "))
	}
	return "", nil
}

func (p *StdioMCPProvider) callToolWithQuery(ctx context.Context, tool mcp.Tool, query string) (string, error) {
	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = tool.Name
	callReq.Params.Arguments = buildToolArguments(tool, query)

	res, err := p.cli.CallTool(ctx, callReq)
	if err != nil {
		return "", fmt.Errorf("call mcp tool failed: %w", err)
	}
	if res.IsError {
		return "", fmt.Errorf("mcp tool returned error")
	}

	var parts []string
	for _, c := range res.Content {
		switch v := c.(type) {
		case mcp.TextContent:
			if strings.TrimSpace(v.Text) != "" {
				parts = append(parts, strings.TrimSpace(v.Text))
			}
		default:
			if b, e := json.Marshal(v); e == nil && len(b) > 0 {
				parts = append(parts, string(b))
			}
		}
	}
	if res.StructuredContent != nil {
		if s := extractTextFromAny(res.StructuredContent); strings.TrimSpace(s) != "" {
			parts = append(parts, strings.TrimSpace(s))
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func buildToolArguments(tool mcp.Tool, query string) map[string]interface{} {
	props := tool.InputSchema.Properties
	if len(props) == 0 {
		return map[string]interface{}{"query": query}
	}

	keys := []string{"query", "q", "text", "prompt", "input", "keyword", "question", "content"}
	for _, k := range keys {
		if _, ok := props[k]; ok {
			return map[string]interface{}{k: query}
		}
	}

	for k := range props {
		return map[string]interface{}{k: query}
	}

	return map[string]interface{}{"query": query}
}

func rankTools(tools []mcp.Tool) []mcp.Tool {
	type scored struct {
		tool  mcp.Tool
		score int
	}
	scoredTools := make([]scored, 0, len(tools))
	for _, t := range tools {
		name := strings.ToLower(t.Name)
		desc := strings.ToLower(t.Description)
		s := 0
		if strings.Contains(name, "context") || strings.Contains(desc, "context") {
			s += 10
		}
		if strings.Contains(name, "search") || strings.Contains(desc, "search") {
			s += 8
		}
		if strings.Contains(name, "retrieve") || strings.Contains(desc, "retrieve") {
			s += 8
		}
		if strings.Contains(name, "query") || strings.Contains(desc, "query") {
			s += 6
		}
		if strings.Contains(name, "knowledge") || strings.Contains(desc, "knowledge") {
			s += 6
		}
		if strings.Contains(name, "rag") || strings.Contains(desc, "rag") {
			s += 4
		}
		scoredTools = append(scoredTools, scored{tool: t, score: s})
	}

	sort.SliceStable(scoredTools, func(i, j int) bool {
		return scoredTools[i].score > scoredTools[j].score
	})

	out := make([]mcp.Tool, 0, len(scoredTools))
	for _, s := range scoredTools {
		out = append(out, s.tool)
	}
	return out
}

func extractTextFromBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	var anyJSON interface{}
	if err := json.Unmarshal(body, &anyJSON); err == nil {
		if text := extractTextFromAny(anyJSON); text != "" {
			return text
		}
	}

	return trimmed
}

func extractTextFromAny(v interface{}) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case []interface{}:
		parts := make([]string, 0, len(x))
		for _, it := range x {
			if s := extractTextFromAny(it); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case map[string]interface{}:
		for _, key := range []string{"context", "text", "result", "content", "answer", "message", "data"} {
			if val, ok := x[key]; ok {
				if s := extractTextFromAny(val); s != "" {
					return s
				}
			}
		}
		if b, err := json.Marshal(x); err == nil {
			return string(b)
		}
	}
	return ""
}

func NewMCPProviderFromEndpoint(endpoint string) (MCPContextProvider, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return NoopMCPProvider{}, nil
	}

	lower := strings.ToLower(endpoint)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return NewHTTPMCPProvider(endpoint)
	}

	if strings.HasPrefix(lower, "stdio:") {
		endpoint = strings.TrimSpace(endpoint[len("stdio:"):])
	}

	parts := strings.Fields(endpoint)
	if len(parts) == 0 {
		return NoopMCPProvider{}, nil
	}

	return NewStdioMCPProvider(parts[0], parts[1:])
}

func (p *StdioMCPProvider) Close() error {
	if p.cli != nil {
		return p.cli.Close()
	}
	return nil
}
