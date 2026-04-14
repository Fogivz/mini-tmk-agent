package asr

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	BaseURL string
	Timeout time.Duration
}

func NewClient() *Client {
	return &Client{
		BaseURL: "http://localhost:8000",
		Timeout: 60 * time.Second,
	}
}

type request struct {
	ReqID      string `json:"req_id,omitempty"`
	SpeakerID  string `json:"speaker_id,omitempty"`
	SourceLang string `json:"source_lang"`
	AudioBase  string `json:"audio_base64"`
}

type response struct {
	Text  string `json:"text"`
	Error string `json:"error,omitempty"`
}

func (c *Client) Transcribe(
	filePath string,
	sourceLang string,
) (string, error) {
	audioBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read audio file failed: %w", err)
	}

	if sourceLang == "" {
		sourceLang = "auto"
	}

	wsURL, err := buildWSURL(c.BaseURL)
	if err != nil {
		return "", err
	}

	dialer := websocket.Dialer{HandshakeTimeout: c.Timeout}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return "", fmt.Errorf("connect asr websocket failed: %w", err)
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(c.Timeout)); err != nil {
		return "", fmt.Errorf("set read deadline failed: %w", err)
	}

	payload := request{
		SourceLang: sourceLang,
		AudioBase:  base64.StdEncoding.EncodeToString(audioBytes),
	}
	if err := conn.WriteJSON(payload); err != nil {
		return "", fmt.Errorf("write websocket payload failed: %w", err)
	}

	var result response
	if err := conn.ReadJSON(&result); err != nil {
		return "", fmt.Errorf("read websocket response failed: %w", err)
	}

	if strings.TrimSpace(result.Error) != "" {
		return "", fmt.Errorf("asr service error: %s", result.Error)
	}

	return result.Text, nil
}

func buildWSURL(baseURL string) (string, error) {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = "http://localhost:8000"
	}

	if !strings.Contains(base, "://") {
		base = "http://" + base
	}

	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid asr base url: %w", err)
	}

	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// keep as is
	default:
		return "", fmt.Errorf("unsupported asr url scheme: %s", u.Scheme)
	}

	basePath := strings.TrimRight(u.Path, "/")
	u.Path = basePath + "/ws/transcribe"

	return u.String(), nil
}
