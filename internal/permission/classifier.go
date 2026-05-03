package permission

import (
	"context"
	"regexp"
)

// LLMClassifier optional LLM fallback interface
type LLMClassifier interface {
	Classify(ctx context.Context, command string) Classification
}

type rule struct {
	pattern *regexp.Regexp
	level   RiskLevel
}

// Classifier classifies command risk level
type Classifier struct {
	rules []rule
	llm   LLMClassifier
}

// NewClassifier creates classifier; llm may be nil
func NewClassifier(llm LLMClassifier) *Classifier {
	return &Classifier{rules: buildStaticRules(), llm: llm}
}

func buildStaticRules() []rule {
	patterns := []struct {
		pattern string
		level   RiskLevel
	}{
		// L4
		{`^rm\s+-[a-zA-Z]*r[a-zA-Z]*f`, L4Destroy},
		{`^rm\s+-[a-zA-Z]*f[a-zA-Z]*r`, L4Destroy},
		{`^dd\s+`, L4Destroy},
		{`^mkfs`, L4Destroy},
		{`^fdisk\s+`, L4Destroy},
		{`^parted\s+`, L4Destroy},
		{`^shred\s+`, L4Destroy},
		// L3
		{`^rm\s+`, L3Dangerous},
		{`^rmdir\s+`, L3Dangerous},
		{`^systemctl\s+stop\s+`, L3Dangerous},
		{`^service\s+\S+\s+stop`, L3Dangerous},
		{`^kill\s+`, L3Dangerous},
		{`^pkill\s+`, L3Dangerous},
		{`^killall\s+`, L3Dangerous},
		{`^truncate\s+`, L3Dangerous},
		{`^>\s+\S+`, L3Dangerous},
		{`^unlink\s+`, L3Dangerous},
		// L2
		{`^echo\s+.*>`, L2Write},
		{`^tee\s+`, L2Write},
		{`^cp\s+`, L2Write},
		{`^mv\s+`, L2Write},
		{`^chmod\s+`, L2Write},
		{`^chown\s+`, L2Write},
		{`^mkdir\s+`, L2Write},
		{`^touch\s+`, L2Write},
		{`^systemctl\s+restart\s+`, L2Write},
		{`^systemctl\s+start\s+`, L2Write},
		{`^service\s+\S+\s+restart`, L2Write},
		{`^service\s+\S+\s+start`, L2Write},
		{`^apt(-get)?\s+install`, L2Write},
		{`^yum\s+install`, L2Write},
		{`^pip\s+install`, L2Write},
		// L1
		{`^ls(\s+|$)`, L1Read},
		{`^cat\s+`, L1Read},
		{`^less\s+`, L1Read},
		{`^more\s+`, L1Read},
		{`^head\s+`, L1Read},
		{`^tail\s+`, L1Read},
		{`^ps(\s+|$)`, L1Read},
		{`^df(\s+|$)`, L1Read},
		{`^du(\s+|$)`, L1Read},
		{`^ping\s+`, L1Read},
		{`^grep\s+`, L1Read},
		{`^find\s+`, L1Read},
		{`^which\s+`, L1Read},
		{`^whoami$`, L1Read},
		{`^hostname$`, L1Read},
		{`^uname(\s+|$)`, L1Read},
		{`^uptime$`, L1Read},
		{`^free(\s+|$)`, L1Read},
		{`^top(\s+|$)`, L1Read},
		{`^htop$`, L1Read},
		{`^journalctl(\s+|$)`, L1Read},
		{`^systemctl\s+status\s+`, L1Read},
		{`^netstat(\s+|$)`, L1Read},
		{`^ss(\s+|$)`, L1Read},
		{`^curl\s+`, L1Read},
		{`^wget\s+`, L1Read},
	}
	rules := make([]rule, 0, len(patterns))
	for _, p := range patterns {
		rules = append(rules, rule{pattern: regexp.MustCompile(p.pattern), level: p.level})
	}
	return rules
}

// Classify returns risk classification for a command
func (c *Classifier) Classify(ctx context.Context, command string) Classification {
	for _, r := range c.rules {
		if r.pattern.MatchString(command) {
			return Classification{Level: r.level, Source: SourceStatic, Reason: "matched: " + r.pattern.String()}
		}
	}
	if c.llm != nil {
		return c.llm.Classify(ctx, command)
	}
	return Classification{Level: L3Dangerous, Source: SourceDefault, Reason: "unknown command, defaulting to L3"}
}
