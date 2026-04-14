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

var rtcCmd = &cobra.Command{
	Use:   "rtc",
	Short: "Run RTC terminal-to-terminal translation mode",
	Long:  "Join an Agora RTC channel and run terminal-to-terminal real-time translation using ASR + DeepSeek + RTC DataStream",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := rtc.Config{
			Role:       strings.ToLower(strings.TrimSpace(rtcRole)),
			SourceLang: strings.TrimSpace(rtcSourceLang),
			TargetLang: strings.TrimSpace(rtcTargetLang),
			AppID:      strings.TrimSpace(rtcAppID),
			AppCert:    strings.TrimSpace(rtcAppCert),
			Token:      strings.TrimSpace(rtcToken),
			Channel:    strings.TrimSpace(rtcChannel),
			UID:        strings.TrimSpace(rtcUID),
			ASRBaseURL: strings.TrimSpace(rtcASRBaseURL),
			TTSCommand: strings.TrimSpace(rtcTTSCommand),
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
	rtcCmd.Flags().StringVar(&rtcTTSCommand, "tts-command", "", "optional shell command executed on receiver when text arrives; receives env vars TTS_TEXT/TTS_LANG/TTS_FROM_UID")

	rootCmd.AddCommand(rtcCmd)
}
