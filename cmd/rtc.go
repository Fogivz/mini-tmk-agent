//go:build rtc

package cmd

import (
	"fmt"
	"strings"

	"go-trans/internal/rtc"

	"github.com/spf13/cobra"
)

var rtcRole string
var rtcSourceLang string
var rtcTargetLang string
var rtcAppID string
var rtcAppCert string
var rtcToken string
var rtcChannel string
var rtcUID string
var rtcASRBaseURL string
var rtcTTSCommand string
var rtcAutoStartASR bool
var rtcASRStartCmd string
var rtcEnableAgent bool
var rtcAgentKnowledgeDir string
var rtcAgentReportDir string
var rtcMCPContextURL string

var rtcCmd = &cobra.Command{
	Use:   "rtc",
	Short: "Run RTC terminal-to-terminal translation mode",
	Long:  "Join an Agora RTC channel and run terminal-to-terminal real-time translation using ASR + DeepSeek + RTC DataStream",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := rtc.Config{
			Role:         strings.ToLower(strings.TrimSpace(rtcRole)),
			SourceLang:   strings.TrimSpace(rtcSourceLang),
			TargetLang:   strings.TrimSpace(rtcTargetLang),
			AppID:        strings.TrimSpace(rtcAppID),
			AppCert:      strings.TrimSpace(rtcAppCert),
			Token:        strings.TrimSpace(rtcToken),
			Channel:      strings.TrimSpace(rtcChannel),
			UID:          strings.TrimSpace(rtcUID),
			ASRBaseURL:   strings.TrimSpace(rtcASRBaseURL),
			TTSCommand:   strings.TrimSpace(rtcTTSCommand),
			AutoStartASR: rtcAutoStartASR,
			ASRStartCmd:  strings.TrimSpace(rtcASRStartCmd),

			EnableAgent:       rtcEnableAgent,
			AgentKnowledgeDir: strings.TrimSpace(rtcAgentKnowledgeDir),
			AgentReportDir:    strings.TrimSpace(rtcAgentReportDir),
			MCPContextURL:     strings.TrimSpace(rtcMCPContextURL),
		}

		if err := rtc.Run(cfg); err != nil {
			fmt.Println("RTC mode error:", err)
		}
	},
}

func init() {
	rtcCmd.Flags().StringVar(&rtcRole, "role", "sender", "rtc role: sender, receiver or duplex")
	rtcCmd.Flags().StringVar(&rtcSourceLang, "source-lang", "zh", "source language")
	rtcCmd.Flags().StringVar(&rtcTargetLang, "target-lang", "en", "target language")
	rtcCmd.Flags().StringVar(&rtcAppID, "app-id", "", "Agora App ID (or AGORA_APP_ID)")
	rtcCmd.Flags().StringVar(&rtcAppCert, "app-cert", "", "Agora App Certificate (or AGORA_APP_CERT)")
	rtcCmd.Flags().StringVar(&rtcToken, "token", "", "Agora RTC token (optional, auto-generated when empty)")
	rtcCmd.Flags().StringVar(&rtcChannel, "channel", "", "Agora channel name (or AGORA_CHANNEL)")
	rtcCmd.Flags().StringVar(&rtcUID, "uid", "", "Agora user id/account (or AGORA_UID)")
	rtcCmd.Flags().StringVar(&rtcASRBaseURL, "asr-url", "http://localhost:8000", "ASR service base URL")
	rtcCmd.Flags().BoolVar(&rtcAutoStartASR, "asr-auto-start", true, "auto start ASR service when unreachable")
	rtcCmd.Flags().StringVar(&rtcASRStartCmd, "asr-start-cmd", "./scripts/start_asr.sh", "command used to start ASR when auto-start is enabled")
	rtcCmd.Flags().StringVar(&rtcTTSCommand, "tts-command", "", "optional shell command executed on receiver when text arrives; receives env vars TTS_TEXT/TTS_LANG/TTS_FROM_UID")
	rtcCmd.Flags().BoolVar(&rtcEnableAgent, "agent", true, "enable session agent features (RAG + Skills + summary report)")
	rtcCmd.Flags().StringVar(&rtcAgentKnowledgeDir, "agent-knowledge-dir", "knowledge", "knowledge directory for local RAG retrieval")
	rtcCmd.Flags().StringVar(&rtcAgentReportDir, "agent-report-dir", "reports", "directory to store generated session reports")
	rtcCmd.Flags().StringVar(&rtcMCPContextURL, "agent-mcp-url", "", "optional MCP source: HTTP endpoint (supports ?query=...) or stdio command (e.g. 'npx -y @modelcontextprotocol/server-memory')")

	rootCmd.AddCommand(rtcCmd)
}
