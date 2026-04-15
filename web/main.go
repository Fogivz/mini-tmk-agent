package main

import (
	"fmt"
	"go-trans/internal/agent"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for demo
	},
}

func main() {
	r := gin.Default()

	// Serve static files
	r.Static("/static", "./static")

	// Home page
	r.GET("/", func(c *gin.Context) {
		c.File("./index.html")
	})

	// Stream mode WebSocket
	r.GET("/ws/stream", func(c *gin.Context) {
		sourceLang := c.Query("source")
		targetLang := c.Query("target")

		if sourceLang == "" {
			sourceLang = "zh"
		}
		if targetLang == "" {
			targetLang = "en"
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("Failed to upgrade to WebSocket:", err)
			return
		}
		defer conn.Close()

		// Create agent with callback
		agent := agent.NewInterpreterAgentWithCallback(
			agent.StreamMode,
			sourceLang,
			targetLang,
			"",
			"",
			func(result string) {
				conn.WriteMessage(websocket.TextMessage, []byte(result))
			},
		)

		// Start agent in goroutine and ensure it's fully finished before returning
		done := make(chan struct{})
		go func() {
			agent.Run()
			close(done)
		}()

		// Keep connection alive and stop on close
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}

		agent.Stop()
		<-done
	})

	// Transcript mode
	r.POST("/transcript", func(c *gin.Context) {
		file, err := c.FormFile("audio")
		if err != nil {
			c.JSON(400, gin.H{"error": "No file uploaded"})
			return
		}

		// Save uploaded file
		tempDir := "./temp"
		os.MkdirAll(tempDir, 0755)
		tempFile := filepath.Join(tempDir, file.Filename)
		if err := c.SaveUploadedFile(file, tempFile); err != nil {
			c.JSON(500, gin.H{"error": "Failed to save file"})
			return
		}
		defer os.Remove(tempFile) // Clean up

		// Run transcript
		agent := agent.NewInterpreterAgent(
			agent.TranscriptMode,
			"",
			"",
			tempFile,
			"",
		)

		// Capture output (simplified, in real app use channels)
		// For now, just run and return result
		result := agent.RunTranscript()

		// Parse result
		lines := strings.Split(result, "\n")
		if len(lines) >= 2 {
			originalText := strings.TrimPrefix(lines[0], "原文:")
			translatedText := strings.TrimPrefix(lines[1], "翻译:")
			responseText := fmt.Sprintf("原文: %s\n翻译: %s", originalText, translatedText)
			c.Header("Content-Disposition", "attachment; filename=transcript.txt")
			c.Data(200, "text/plain", []byte(responseText))
		} else {
			c.Header("Content-Disposition", "attachment; filename=transcript.txt")
			c.Data(200, "text/plain", []byte(result))
		}
	})

	fmt.Println("Web UI starting on :8080")
	r.Run(":8080")
}
