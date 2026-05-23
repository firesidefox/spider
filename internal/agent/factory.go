package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/knowledge"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/permission"
	"github.com/spiderai/spider/internal/rag"
	"github.com/spiderai/spider/internal/ssh"
	"github.com/spiderai/spider/internal/store"
)

// Factory holds shared dependencies for creating Agent instances.
type Factory struct {
	LLMClient                    llm.Client
	Hosts                        *store.HostStore
	AccessFaces                  *store.AccessFaceStore
	SSHPool                      *ssh.Pool
	SSHKeys                      *store.SSHKeyStore
	Logs                         *store.LogStore
	MsgStore                     MessageStorer
	Enforcer                     *permission.Enforcer
	PermissionMode               permission.Mode
	SummaryStore                 *store.SummaryStore
	CompactionCfg                config.CompactionConfig
	MaxTurns                     int
	LLMModel                     string
	TodoStore                    *store.TodoStore
	TopologyStore                *store.TopologyStore
	SSEBroadcaster               SSEBroadcaster
	DataDir                      string
	DocStore                     *store.DocumentStore
	RagStore                     *rag.Store
	KnowledgeStore               *knowledge.Store
	Embedder                     rag.Embedder
	TaskStore                    *store.TaskStore
	DisableSearchDocs            bool
	PerToolResultMaxChars        int
	PerMessageToolResultMaxChars int
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

func (f *Factory) maxTurns() int {
	if f.MaxTurns > 0 {
		return f.MaxTurns
	}
	return 10000
}

// NewAgent creates a new Agent with all tools registered.
func (f *Factory) NewAgent(conversationID string, selectedHostIDs []string) *Agent {
	logger.ForModule("agent").Info().Str("model", f.LLMModel).Str("conv_id", conversationID).Msg("agent factory: creating agent")
	registry := f.buildRegistryWithHosts(conversationID, selectedHostIDs)

	hooks := NewHookChain()
	if f.Enforcer != nil {
		hooks.AddBefore(PermissionHook(f.Enforcer, f.PermissionMode))
	} else {
		hooks.AddBefore(DefaultRiskHook())
	}

	var compactor *Compactor
	replacementState := newContentReplacementState()
	if f.SummaryStore != nil {
		compactor = NewCompactor(f.LLMClient, f.SummaryStore, f.MsgStore, f.LLMModel, f.CompactionCfg,
			f.DataDir, f.PerMessageToolResultMaxChars, replacementState)
	}
	return NewAgent(AgentConfig{
		LLMClient:                    f.LLMClient,
		Registry:                     registry,
		Hooks:                        hooks,
		MsgStore:                     f.MsgStore,
		TodoStore:                    f.TodoStore,
		Hosts:                        f.Hosts,
		SystemPrompt:                 f.BuildSystemPrompt(),
		MaxTurns:                     f.maxTurns(),
		Compactor:                    compactor,
		SkillManager:                 NewSkillManager(f.DataDir),
		DataDir:                      f.DataDir,
		PerToolResultMaxChars:        f.PerToolResultMaxChars,
		PerMessageToolResultMaxChars: f.PerMessageToolResultMaxChars,
		ReplacementState:             replacementState,
	})
}

// NewHeadlessAgent creates an Agent that discards all messages (no DB writes).
// Used for automated task runs that don't need conversation history.
func (f *Factory) NewHeadlessAgent(conversationID string, extraDynamic ...string) *Agent {
	logger.ForModule("agent").Info().Str("model", f.LLMModel).Str("conv_id", conversationID).Msg("agent factory: creating headless agent")
	registry := f.buildRegistry(conversationID)

	hooks := NewHookChain()
	if f.Enforcer != nil {
		hooks.AddBefore(PermissionHook(f.Enforcer, f.PermissionMode))
	} else {
		hooks.AddBefore(DefaultRiskHook())
	}

	return NewAgent(AgentConfig{
		LLMClient:                    f.LLMClient,
		Registry:                     registry,
		Hooks:                        hooks,
		MsgStore:                     noopMessageStorer{},
		SystemPrompt:                 f.BuildSystemPrompt(extraDynamic...),
		MaxTurns:                     f.maxTurns(),
		SkillManager:                 NewSkillManager(f.DataDir),
		DataDir:                      f.DataDir,
		PerToolResultMaxChars:        f.PerToolResultMaxChars,
		PerMessageToolResultMaxChars: f.PerMessageToolResultMaxChars,
		ReplacementState:             newContentReplacementState(),
	})
}

const identityPrompt = `You are Spider, an intelligent network operations assistant. Use the available tools to execute CLI commands, verify configurations, query REST APIs, and answer questions about network infrastructure.`

const communicatingPrompt = `## Communicating with the user

Tool calls and tool results are mostly invisible to the user — they see only your text output and the final result. Treat your text as the only reliable channel for explaining what is happening.

**Before your first tool call** in a turn, state in one short sentence what you are about to do.

**During work**, send a short update only at these moments:
- You found something load-bearing (a config error, a root cause, a host that matches the filter).
- You are changing direction based on what you saw.
- A risky / write-class command (RunCommand L2/L3, RunCommandBatch) is about to run — restate the intent and target hosts.

**Do not narrate** routine reads (GetHosts, ListAccessFaces, SearchDocs). Do not echo command output that the UI already renders. Do not write "I will now ...", "Next, I will ..." between every tool call.

**End-of-turn**: one sentence on the result. No bullet recap of every step. If the result is a table or list, the table IS the answer — don't prepend "Here is the result:".`

const toneAndStylePrompt = `## Tone and Style

- Always respond in Simplified Chinese. Use English only for technical terms, command output, and code.
- Be direct. Lead with the result. No pleasantries ("好的", "当然", "我来帮您", "没问题").
- **Intent statements are NOT preamble.** The one-sentence statement before your first tool call ("先列出 cisco 设备查端口状态") is required by the Communicating with the user section. Do not omit it. The distinction: pleasantries are social niceties; intent statements explain the next action.
- Do not use a colon before tool calls. Writing "让我看看：" followed by a tool call becomes a broken sentence when the tool call is not rendered. Rewrite as "让我看看。" + tool call, or state the intent directly.
- For multi-host results, use tables or lists — not prose.
- Reference code with file_path:line_number format (e.g., internal/agent/factory.go:193).
- Reference hosts by hostname, not host_id.
- Do not use emojis unless the user explicitly requests them.`

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

// BuildSystemPrompt constructs the system prompt as two segments:
// Static segment (cacheable): identity + communicating + tone + tool sections + orchestration + intent
// Dynamic segment: environment (host inventory) + optional extraDynamic
func (f *Factory) BuildSystemPrompt(extraDynamic ...string) []llm.SystemBlock {
	var static strings.Builder
	static.WriteString(identityPrompt)
	static.WriteString("\n\n")
	static.WriteString(communicatingPrompt)
	static.WriteString("\n\n")
	static.WriteString(toneAndStylePrompt)
	static.WriteString("\n\n")

	reg := f.buildRegistry("")
	for _, tool := range reg.All() {
		if sp, ok := tool.(SystemPromptSection); ok {
			section := sp.SystemPromptSection()
			if strings.TrimSpace(section) != "" {
				static.WriteString(section)
				static.WriteString("\n\n")
			}
		}
	}

	static.WriteString(orchestrationPrompt)
	static.WriteString("\n\n")
	static.WriteString(intentFieldPrompt)

	var dynamicBuilder strings.Builder
	dynamicBuilder.WriteString(f.buildEnvironmentSection())
	for _, extra := range extraDynamic {
		dynamicBuilder.WriteString("\n\n")
		dynamicBuilder.WriteString(extra)
	}

	cacheMark := "ephemeral"
	return []llm.SystemBlock{
		{Text: static.String(), CacheControl: &cacheMark},
		{Text: dynamicBuilder.String()},
	}
}

func (f *Factory) buildEnvironmentSection() string {
	allHosts, err := f.Hosts.List("")
	if err != nil || len(allHosts) == 0 {
		return "## Environment\n\nNo hosts are currently registered."
	}
	vendorCount := make(map[string]int)
	for _, h := range allHosts {
		v := h.Vendor
		if v == "" {
			v = "unknown"
		}
		vendorCount[v]++
	}
	// Sort by vendor name to avoid map iteration non-determinism
	vendors := make([]string, 0, len(vendorCount))
	for v := range vendorCount {
		vendors = append(vendors, v)
	}
	sort.Strings(vendors)
	var parts []string
	for _, v := range vendors {
		parts = append(parts, fmt.Sprintf("%s(%d)", v, vendorCount[v]))
	}
	return fmt.Sprintf(
		"## Environment\n\nManaged devices: %d total — %s.",
		len(allHosts), strings.Join(parts, ", "),
	)
}

// buildRegistry creates a temporary registry to collect tool SystemPromptSections.
func (f *Factory) buildRegistry(conversationID string) *ToolRegistry {
	return f.buildRegistryWithHosts(conversationID, nil)
}

func (f *Factory) buildRegistryWithHosts(conversationID string, selectedHostIDs []string) *ToolRegistry {
	registry := NewToolRegistry()
	listTool := NewGetHostsTool(f.Hosts, f.AccessFaces)
	listTool.selectedHostIDs = selectedHostIDs
	listTool.knowledgeStore = f.KnowledgeStore
	registry.Register(listTool)
	registry.Register(NewExecuteCLITool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewBatchExecuteTool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewVerifyTool(f.Hosts, f.AccessFaces, f.SSHPool, f.SSHKeys))
	registry.Register(NewCallRESTAPITool(f.AccessFaces))
	registry.Register(NewSearchDocsTool(f.KnowledgeStore, f.Embedder))
	registry.Register(NewTodoTool(f.TodoStore, f.SSEBroadcaster, conversationID))
	registry.Register(NewGetTopologyTool(f.TopologyStore))
	registry.Register(NewGetTopologyContextTool(f.TopologyStore))
	registry.Register(NewCreateTaskTool(f.TaskStore, conversationID))
	registry.Register(NewInvokeSkillTool(f.DataDir))
	return registry
}
