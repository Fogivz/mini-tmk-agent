package deepseek

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

func (c *Client) Translate(context string, text string, targetLang string) (string, error) {
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

	reqBody := ChatRequest{
		Model: "deepseek-chat",
		Messages: []MessageItem{
			{
				Role: "system",
				Content: "You are a professional simultaneous interpreter. " +
					"Use previous conversation context to improve translation consistency.",
			},
			{
				Role: "user",
				Content: fmt.Sprintf(
					"Previous context:\n%s\n\nTranslate this text to %s:\n%s",
					context,
					targetName,
					text,
				),
			},
		},
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(
		"POST",
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

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response from deepseek")
	}

	return result.Choices[0].Message.Content, nil
}
