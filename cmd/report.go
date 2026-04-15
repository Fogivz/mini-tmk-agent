package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go-trans/internal/agentx"
	"go-trans/internal/deepseek"

	"github.com/spf13/cobra"
)

var reportInput string
var reportOutput string
var reportDir string
var reportKnowledgeDir string
var reportMCPContextURL string
var reportCleanupInput bool

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "离线生成会话报告（输入turn JSON，输出summary/viewpoints）",
	Long:  "读取本地turn JSON文件，生成会话摘要和观点分析报告，便于回放和评估Agent质量。",
	Run: func(cmd *cobra.Command, args []string) {
		if reportInput == "" {
			fmt.Println("Error: 需要 --input <turns.json> 参数")
			os.Exit(1)
		}

		f, err := os.Open(reportInput)
		if err != nil {
			fmt.Println("打开输入文件失败:", err)
			os.Exit(1)
		}
		defer f.Close()

		var turns []agentx.Turn
		if err := json.NewDecoder(f).Decode(&turns); err != nil {
			fmt.Println("解析turn JSON失败:", err)
			os.Exit(1)
		}

		agent := agentx.NewSessionAgent(agentx.Options{
			SessionID:    fmt.Sprintf("offline_%d", time.Now().Unix()),
			KnowledgeDir: reportKnowledgeDir,
			MCPEndpoint:  reportMCPContextURL,
		}, deepseek.NewClient())
		for _, t := range turns {
			agent.AddTurn(t)
		}

		report, err := agent.GenerateReport(context.Background())
		if err != nil {
			fmt.Println("生成报告失败:", err)
			os.Exit(1)
		}

		bs, _ := json.MarshalIndent(report, "", "  ")
		if strings.TrimSpace(reportOutput) != "" {
			if err := os.MkdirAll(filepath.Dir(reportOutput), 0o755); err != nil {
				fmt.Println("创建报告目录失败:", err)
				os.Exit(1)
			}
			if err := os.WriteFile(reportOutput, bs, 0o644); err != nil {
				fmt.Println("写入 JSON 报告失败:", err)
				os.Exit(1)
			}

			mdPath := strings.TrimSuffix(reportOutput, filepath.Ext(reportOutput)) + ".md"
			if err := os.WriteFile(mdPath, []byte(buildReportMarkdown(report)), 0o644); err != nil {
				fmt.Println("写入 Markdown 报告失败:", err)
				os.Exit(1)
			}

			fmt.Println("报告(JSON)已写入:", reportOutput)
			fmt.Println("报告(Markdown)已写入:", mdPath)
		} else {
			dir := strings.TrimSpace(reportDir)
			if dir == "" {
				dir = "reports"
			}
			jsonPath, mdPath, err := writeReportWithAutoName(dir, bs, []byte(buildReportMarkdown(report)))
			if err != nil {
				fmt.Println("写入报告失败:", err)
				os.Exit(1)
			}
			fmt.Println("报告(JSON)已写入:", jsonPath)
			fmt.Println("报告(Markdown)已写入:", mdPath)
		}

		if reportCleanupInput {
			_ = os.Remove(reportInput)
		}
	},
}

func buildReportMarkdown(report agentx.Report) string {
	mdContent := fmt.Sprintf("# 会话记录报告 - %s\n\n## 会话摘要\n\n%s\n\n## 关键观点分析\n\n%s\n\n## 完整对话记录\n\n", report.SessionID, report.Summary, report.Viewpoints)
	for _, turn := range report.Turns {
		ts := time.UnixMilli(turn.TimestampMs).Format("2006-01-02 15:04:05")
		mdContent += fmt.Sprintf("### 时间: %s\n- **%s (原声识别)**: %s\n- **%s (翻译与处理)**: %s\n\n", ts, turn.SpeakerID, turn.OriginalText, turn.SpeakerID, turn.TranslatedText)
	}
	return mdContent
}

func writeReportWithAutoName(dir string, jsonBytes []byte, mdBytes []byte) (string, string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("create report dir failed: %w", err)
	}

	for i := 1; i <= 1000000; i++ {
		base := "session_" + strconv.Itoa(i)
		jsonPath := filepath.Join(dir, base+".json")
		mdPath := filepath.Join(dir, base+".md")

		f, err := os.OpenFile(jsonPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", "", fmt.Errorf("create json report failed: %w", err)
		}
		if _, err := f.Write(jsonBytes); err != nil {
			_ = f.Close()
			_ = os.Remove(jsonPath)
			return "", "", fmt.Errorf("write json report failed: %w", err)
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(jsonPath)
			return "", "", fmt.Errorf("close json report failed: %w", err)
		}

		if err := os.WriteFile(mdPath, mdBytes, 0o644); err != nil {
			_ = os.Remove(jsonPath)
			return "", "", fmt.Errorf("write md report failed: %w", err)
		}

		return jsonPath, mdPath, nil
	}

	return "", "", fmt.Errorf("too many report files, cannot allocate session_n name")
}

func init() {
	reportCmd.Flags().StringVar(&reportInput, "input", "", "输入turn JSON文件路径（必需）")
	reportCmd.Flags().StringVar(&reportOutput, "output", "", "输出报告文件路径（可选，默认打印到stdout）")
	reportCmd.Flags().StringVar(&reportDir, "report-dir", "reports", "自动命名输出目录（默认 reports，输出为 session_n.json/md）")
	reportCmd.Flags().StringVar(&reportKnowledgeDir, "knowledge", "", "RAG知识库目录（可选）")
	reportCmd.Flags().StringVar(&reportMCPContextURL, "mcp", "", "MCP上下文服务URL（可选）")
	reportCmd.Flags().BoolVar(&reportCleanupInput, "cleanup-input", false, "生成完成后删除输入 turns 文件")

	rootCmd.AddCommand(reportCmd)
}
