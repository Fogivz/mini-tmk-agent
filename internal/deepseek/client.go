package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type Client struct {
	APIKey  string
	BaseURL string
}

func NewClient() *Client {
	return &Client{
		APIKey:  os.Getenv("DEEPSEEK_API_KEY"),
		BaseURL: "https://api.deepseek.com/chat/completions",
	}
}

type MessageItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []MessageItem `json:"messages"`
	Stream   bool          `json:"stream"`
}

type ChatResponse struct {
	Choices []struct {
		Message MessageItem `json:"message"`
	} `json:"choices"`
}

func (c *Client) Translate(contextText string, text string, targetLang string) (string, error) {
	langMap := map[string]string{
		"zh": "Chinese",
		"en": "English",
		"ja": "Japanese",
		"ko": "Korean",
		"fr": "French",
		"de": "German",
		"es": "Spanish",
	}

	targetName, exists := langMap[targetLang]
	if !exists {
		targetName = "English"
	}

	resp, err := c.Chat(
		context.Background(),
		"You are a professional simultaneous interpreter. Use previous conversation context to improve translation consistency. Output translation text only, with no explanation or prefixes.",
		fmt.Sprintf(
			"Previous context:\n%s\n\nTranslate this text to %s:\n%s",
			contextText,
			targetName,
			text,
		),
	)
	if err != nil {
		return "", err
	}
	return sanitizeTranslation(resp), nil
}

func (c *Client) Chat(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return "", fmt.Errorf("missing DEEPSEEK_API_KEY")
	}

	reqBody := ChatRequest{
		Model: "deepseek-chat",
		Messages: []MessageItem{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.BaseURL,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("deepseek returned status %d", resp.StatusCode)
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response from deepseek")
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func sanitizeTranslation(s string) string {
	out := strings.TrimSpace(s)
	out = strings.Trim(out, "\"'“”‘’")

	prefixes := []string{
		"翻译成中文：", "翻译成中文:", "翻译为中文：", "翻译为中文:",
		"翻译成英文：", "翻译成英文:", "翻译为英文：", "翻译为英文:",
		"翻译：", "翻译:", "译文：", "译文:",
		"Translation:", "Translate:",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(strings.ToLower(out), strings.ToLower(p)) {
			out = strings.TrimSpace(out[len(p):])
			break
		}
	}

	out = strings.ReplaceAll(out, "\r\n", "\n")
	out = strings.ReplaceAll(out, "\r", "\n")
	out = strings.Join(strings.Fields(out), " ")
	return strings.TrimSpace(out)
}
