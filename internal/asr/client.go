package asr

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	BaseURL   string
	Timeout   time.Duration
	ChunkSize int

	mu     sync.Mutex
	conn   *websocket.Conn
	seq    uint64
	wsURL  string
	dialer websocket.Dialer
}

func NewClient() *Client {
	return &Client{
		BaseURL:   "http://localhost:8000",
		Timeout:   60 * time.Second,
		ChunkSize: 16 * 1024,
		dialer:    websocket.Dialer{HandshakeTimeout: 8 * time.Second},
	}
}

type request struct {
	Type       string `json:"type,omitempty"`
	ReqID      string `json:"req_id,omitempty"`
	SpeakerID  string `json:"speaker_id,omitempty"`
	SourceLang string `json:"source_lang"`
	AudioBase  string `json:"audio_base64,omitempty"`
}

type response struct {
	Type      string `json:"type,omitempty"`
	ReqID     string `json:"req_id,omitempty"`
	SpeakerID string `json:"speaker_id,omitempty"`
	Text      string `json:"text"`
	IsFinal   bool   `json:"is_final,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Event struct {
	Type      string
	ReqID     string
	SpeakerID string
	Text      string
	IsFinal   bool
	Error     string
}

func (c *Client) Transcribe(
	filePath string,
	sourceLang string,
) (string, error) {
	return c.TranscribeStream(filePath, sourceLang, nil)
}

func (c *Client) TranscribeStream(
	filePath string,
	sourceLang string,
	onEvent func(Event),
) (string, error) {
	audioBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read audio file failed: %w", err)
	}

	if sourceLang == "" {
		sourceLang = "auto"
	}

	reqID := c.nextReqID()

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureConnLocked(); err != nil {
		return "", err
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(c.Timeout)); err != nil {
		c.resetConnLocked()
		return "", fmt.Errorf("set read deadline failed: %w", err)
	}

	if err := c.conn.WriteJSON(request{Type: "start", ReqID: reqID, SourceLang: sourceLang}); err != nil {
		c.resetConnLocked()
		return "", fmt.Errorf("send asr start failed: %w", err)
	}

	chunkSize := c.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 16 * 1024
	}

	for start := 0; start < len(audioBytes); start += chunkSize {
		end := start + chunkSize
		if end > len(audioBytes) {
			end = len(audioBytes)
		}

		if err := c.conn.WriteJSON(request{
			Type:      "chunk",
			ReqID:     reqID,
			AudioBase: base64.StdEncoding.EncodeToString(audioBytes[start:end]),
		}); err != nil {
			c.resetConnLocked()
			return "", fmt.Errorf("send asr chunk failed: %w", err)
		}
	}

	if err := c.conn.WriteJSON(request{Type: "end", ReqID: reqID}); err != nil {
		c.resetConnLocked()
		return "", fmt.Errorf("send asr end failed: %w", err)
	}

	finalText := ""
	for {
		var result response
		if err := c.conn.ReadJSON(&result); err != nil {
			c.resetConnLocked()
			return "", fmt.Errorf("read websocket response failed: %w", err)
		}

		if strings.TrimSpace(result.ReqID) != "" && result.ReqID != reqID {
			continue
		}

		event := Event{
			Type:      result.Type,
			ReqID:     result.ReqID,
			SpeakerID: result.SpeakerID,
			Text:      result.Text,
			IsFinal:   result.IsFinal || strings.EqualFold(result.Type, "final"),
			Error:     result.Error,
		}

		if onEvent != nil {
			onEvent(event)
		}

		if strings.TrimSpace(event.Error) != "" {
			return "", fmt.Errorf("asr service error: %s", event.Error)
		}

		if strings.EqualFold(event.Type, "partial") {
			continue
		}

		if event.IsFinal || strings.EqualFold(event.Type, "final") || event.Type == "" {
			finalText = strings.TrimSpace(event.Text)
			break
		}
	}

	return finalText, nil
}

func (c *Client) ensureConnLocked() error {
	wsURL, err := buildWSURL(c.BaseURL)
	if err != nil {
		return err
	}

	if c.conn != nil && c.wsURL == wsURL {
		return nil
	}

	c.resetConnLocked()
	conn, _, err := c.dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("connect asr websocket failed: %w", err)
	}
	c.conn = conn
	c.wsURL = wsURL
	return nil
}

func (c *Client) resetConnLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.wsURL = ""
}

func (c *Client) nextReqID() string {
	n := atomic.AddUint64(&c.seq, 1)
	return fmt.Sprintf("asr-%d-%d", time.Now().UnixMilli(), n)
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
