package agent

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/permission"
	"github.com/spiderai/spider/internal/rag"
	"github.com/spiderai/spider/internal/ssh"
	"github.com/spiderai/spider/internal/store"
)

// Factory holds shared dependencies for creating Agent instances.
type Factory struct {
	LLMClient      llm.Client
	Hosts          *store.HostStore
	AccessFaces    *store.AccessFaceStore
	SSHPool        *ssh.Pool
	SSHKeys        *store.SSHKeyStore
	Logs           *store.LogStore
	MsgStore       MessageStorer
	Enforcer       *permission.Enforcer
	PermissionMode permission.Mode
	SummaryStore   *store.SummaryStore
	CompactionCfg  config.CompactionConfig
	LLMModel       string
	TodoStore      *store.TodoStore
	TopologyStore  *store.TopologyStore
	SSEBroadcaster SSEBroadcaster
	DataDir        string
	DocStore       *store.DocumentStore
	RagStore       *rag.Store
	TaskStore      *store.TaskStore
}

// NewFactory creates a Factory by reading the active provider from the DB.
func NewFactory(
	providerStore *store.ProviderStore,
	hosts *store.HostStore,
	faces *store.AccessFaceStore,
	pool *ssh.Pool,
	keys *store.SSHKeyStore,
	logs *store.LogStore,
	msgs MessageStorer,
) (*Factory, error) {
	provider, err := providerStore.GetActive()
	if err != nil {
		return nil, fmt.Errorf("get active provider: %w", err)
	}
	if provider == nil {
		return nil, fmt.Errorf("no active provider configured")
	}
	if provider.SelectedModel == "" {
		return nil, fmt.Errorf("no model selected for provider %s", provider.Name)
	}

	apiKey, err := providerStore.DecryptAPIKey(provider)
	if err != nil {
		return nil, fmt.Errorf("decrypt API key: %w", err)
	}

	llmClient, err := llm.NewClient(provider.Type, apiKey, provider.SelectedModel, provider.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	return &Factory{
		LLMClient:   llmClient,
		Hosts:       hosts,
		AccessFaces: faces,
		SSHPool:     pool,
		SSHKeys:     keys,
		Logs:        logs,
		MsgStore:    msgs,
		LLMModel:    provider.SelectedModel,
	}, nil
}

// NewAgent creates a new Agent with all tools registered.
func (f *Factory) NewAgent(systemPrompt string, conversationID string) *Agent {
	logger.Global().Info().Str("model", f.LLMModel).Str("conv_id", conversationID).Msg("agent factory: creating agent")
	registry := f.buildRegistry(conversationID)

	hooks := NewHookChain()
	if f.Enforcer != nil {
		hooks.AddBefore(PermissionHook(f.Enforcer, f.PermissionMode))
	} else {
		hooks.AddBefore(DefaultRiskHook())
	}

	var compactor *Compactor
	if f.SummaryStore != nil {
		compactor = NewCompactor(f.LLMClient, f.SummaryStore, f.MsgStore, f.LLMModel, f.CompactionCfg)
	}
	return NewAgent(AgentConfig{
		LLMClient:    f.LLMClient,
		Registry:     registry,
		Hooks:        hooks,
		MsgStore:     f.MsgStore,
		SystemPrompt: systemPrompt,
		MaxTurns:     15,
		Compactor:    compactor,
		SkillManager: NewSkillManager(f.DataDir),
	})
}

// NewHeadlessAgent creates an Agent that discards all messages (no DB writes).
// Used for automated task runs that don't need conversation history.
func (f *Factory) NewHeadlessAgent(systemPrompt string, conversationID string) *Agent {
	logger.Global().Info().Str("model", f.LLMModel).Str("conv_id", conversationID).Msg("agent factory: creating headless agent")
	registry := f.buildRegistry(conversationID)

	hooks := NewHookChain()
	if f.Enforcer != nil {
		hooks.AddBefore(PermissionHook(f.Enforcer, f.PermissionMode))
	} else {
		hooks.AddBefore(DefaultRiskHook())
	}

	return NewAgent(AgentConfig{
		LLMClient:    f.LLMClient,
		Registry:     registry,
		Hooks:        hooks,
		MsgStore:     noopMessageStorer{},
		SystemPrompt: systemPrompt,
		MaxTurns:     15,
		SkillManager: NewSkillManager(f.DataDir),
	})
}

const intentFieldPrompt = `## Intent Field (RunCommand / RunCommandBatch / CallAPI)

Always set the intent field. This field is shown to the user in the UI.

**Rules:**
- Write the goal only — do not include device names (the UI adds those automatically)
- Keep it short: 10 Chinese characters or fewer is ideal

<example>
Good: "重启 nginx 使配置生效"
Good: "清理 30 天前的日志"
Bad: "在 local110 和 local201 上重启 nginx" — device names belong in host_ids, not intent
</example>`

const orchestrationPrompt = `## Complex Multi-Step Tasks

**Explore → Plan → Confirm → Act → Verify**

**Dependency chain:** If a step fails, stop. Report what failed before asking how to continue.

**Conditional branching:** Gather facts in Explore phase first. Pick one path based on data — do not execute branches speculatively.

<example>
User: Optimize the web server response time.
Assistant: Collects CPU, memory, and I/O metrics first. Then picks one optimization path based on the bottleneck — does not apply all optimizations at once.
</example>

**Verification:** After each Act step, verify before marking completed. If verification fails, keep in_progress and offer rollback if available.`

// BuildSystemPrompt builds a three-layer system prompt.
// Layer 1: dynamic role + host summary
// Layer 2: per-tool SystemPromptSection() collected from a temp registry
// Layer 3: intentFieldPrompt + orchestrationPrompt
func (f *Factory) BuildSystemPrompt() string {
	// Layer 1
	allHosts, err := f.Hosts.List("")
	var layer1 string
	if err != nil || len(allHosts) == 0 {
		layer1 = "You are Spider, an intelligent network operations assistant. No hosts are currently registered."
	} else {
		vendorCount := make(map[string]int)
		for _, h := range allHosts {
			v := h.Vendor
			if v == "" {
				v = "unknown"
			}
			vendorCount[v]++
		}
		var parts []string
		for vendor, count := range vendorCount {
			parts = append(parts, fmt.Sprintf("%s(%d)", vendor, count))
		}
		layer1 = fmt.Sprintf(
			"You are Spider, an intelligent network operations assistant. "+
				"You manage %d network devices: %s. "+
				"Use the available tools to execute CLI commands, verify configurations, "+
				"and answer questions about the network infrastructure.",
			len(allHosts),
			strings.Join(parts, ", "),
		)
	}

	// Layer 2: collect tool sections
	reg := f.buildRegistry("")
	var b strings.Builder
	b.WriteString(layer1)
	for _, tool := range reg.All() {
		if sp, ok := tool.(SystemPromptSection); ok {
			section := sp.SystemPromptSection()
			if strings.TrimSpace(section) != "" {
				b.WriteString("\n\n")
				b.WriteString(section)
			}
		}
	}

	// Layer 3
	b.WriteString("\n\n")
	b.WriteString(intentFieldPrompt)
	b.WriteString("\n\n")
	b.WriteString(orchestrationPrompt)
	b.WriteString("\n\n## Language\n\nAlways respond in Chinese (Simplified). Use English only for technical terms, command output, and code.")
	return b.String()
}

// buildRegistry creates a temporary registry to collect tool SystemPromptSections.
func (f *Factory) buildRegistry(conversationID string) *ToolRegistry {
	registry := NewToolRegistry()
	registry.Register(NewListDevicesTool(f.Hosts, f.AccessFaces))
	registry.Register(NewExecuteCLITool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewBatchExecuteTool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewVerifyTool(f.Hosts, f.AccessFaces, f.SSHPool, f.SSHKeys))
	registry.Register(NewCallRESTAPITool(f.AccessFaces))
	registry.Register(NewSearchDocsTool(f.RagStore, f.DocStore))
	if f.TodoStore != nil {
		registry.Register(NewTodoTool(f.TodoStore, f.SSEBroadcaster, conversationID))
	}
	if f.TopologyStore != nil {
		registry.Register(NewGetTopologyTool(f.TopologyStore))
	}
	if f.TaskStore != nil {
		registry.Register(NewCreateTaskTool(f.TaskStore, conversationID))
	}
	if f.DataDir != "" {
		registry.Register(NewInvokeSkillTool(filepath.Join(f.DataDir, "skills")))
	}
	return registry
}
