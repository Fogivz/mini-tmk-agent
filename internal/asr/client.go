package asr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient() *Client {
	return &Client{
		BaseURL: "http://localhost:8000",
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type Response struct {
	Text string `json:"text"`
}

func (c *Client) Transcribe(
	filePath string,
	sourceLang string,
) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open audio file failed: %w", err)
	}
	defer file.Close()

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	part, err := writer.CreateFormFile("file", filePath)
	if err != nil {
		return "", fmt.Errorf("create form file failed: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return "", fmt.Errorf("copy audio content failed: %w", err)
	}

	if sourceLang == "" {
		sourceLang = "auto"
	}

	err = writer.WriteField("source_lang", sourceLang)
	if err != nil {
		return "", fmt.Errorf("write source_lang failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer failed: %w", err)
	}

	req, err := http.NewRequest(
		"POST",
		c.BaseURL+"/transcribe",
		&requestBody,
	)
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request asr service failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf(
			"asr service returned %d: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response failed: %w", err)
	}

	return result.Text, nil
}
