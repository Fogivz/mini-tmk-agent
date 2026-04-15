package agentx

import (
	"context"
	"fmt"
	"strings"
)

type Skill interface {
	Name() string
	Run(ctx context.Context, agent *SessionAgent) (string, error)
}

const (
	SkillSummary    = "summary"
	SkillViewpoints = "viewpoints"
)

type SummarySkill struct{}

func (SummarySkill) Name() string { return SkillSummary }

func (SummarySkill) Run(ctx context.Context, agent *SessionAgent) (string, error) {
	prompt := agent.buildSharedPrompt(ctx, "请生成会话摘要，要求：\n1) 100-200字\n2) 重点保留结论、分歧、待办\n3) 使用中文输出")
	return agent.llm.Chat(ctx,
		"你是会议纪要助手，擅长压缩信息并保持事实准确。",
		prompt,
	)
}

type ViewpointsSkill struct{}

func (ViewpointsSkill) Name() string { return SkillViewpoints }

func (ViewpointsSkill) Run(ctx context.Context, agent *SessionAgent) (string, error) {
	prompt := agent.buildSharedPrompt(ctx, "请提炼双方观点。输出格式：\n甲方观点:\n- ...\n乙方观点:\n- ...\n共识:\n- ...\n争议:\n- ...")
	return agent.llm.Chat(ctx,
		"你是辩论分析助手，擅长抽取观点并区分事实与立场。",
		prompt,
	)
}

func (a *SessionAgent) runSkill(ctx context.Context, name string) (string, error) {
	skill, ok := a.skills[name]
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	return skill.Run(ctx, a)
}

func formatRag(snippets []RagSnippet) string {
	if len(snippets) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(snippets))
	for _, s := range snippets {
		parts = append(parts, fmt.Sprintf("来源:%s\n相关度:%d\n%s", s.Path, s.Score, s.Content))
	}
	return strings.Join(parts, "\n\n")
}
