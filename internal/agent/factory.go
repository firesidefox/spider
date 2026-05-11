package agent

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/permission"
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
	TodoTaskStore  *store.TodoTaskStore
	SSEBroadcaster SSEBroadcaster
	DataDir        string
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
	registry := NewToolRegistry()
	registry.Register(NewListDevicesTool(f.Hosts))
	registry.Register(NewGetDeviceInfoTool(f.Hosts))
	registry.Register(NewExecuteCLITool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewBatchExecuteTool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewVerifyTool(f.Hosts, f.AccessFaces, f.SSHPool, f.SSHKeys))
	registry.Register(NewCallRESTAPITool(f.AccessFaces))
	if f.TodoTaskStore != nil {
		registry.Register(NewTodoTaskTool(f.TodoTaskStore, f.SSEBroadcaster, conversationID))
	}
	if f.DataDir != "" {
		registry.Register(NewInvokeSkillTool(filepath.Join(f.DataDir, "skills")))
	}

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

const toolBehaviorPrompt = `

## Tool Usage Guidelines

### ListDevices / GetDeviceInfo / SearchDocs (read-only, no side effects)

**When to use:** Call these freely at the start of any task to understand the environment.

<exemple>
User: Check disk usage on all web servers.
Assistant: Calls ListDevices to find web servers before running any commands.
<reasoning>
Need to know which hosts exist before targeting them. Read-only — no cost to calling early.
</reasoning>
</exemple>

### VerifyTool (read-only, has retry semantics)

**When to use:** After a deployment or config change, to poll until a service is ready.
**When NOT to use:** Don't use for a one-shot check — use RunCommand instead. VerifyTool retries on failure, adding latency when you just need a single result.

<exemple>
User: Restart nginx and confirm it's up.
Assistant: Calls RunCommand to restart, then VerifyTool to poll until nginx responds.
<reasoning>
VerifyTool's retry loop is the right tool for "wait until ready" — not for "check once".
</reasoning>
</exemple>

<exemple>
User: Is port 80 open on web-01?
Assistant: Calls RunCommand with "ss -tlnp | grep :80". Does NOT use VerifyTool.
<reasoning>
One-shot check. VerifyTool would retry unnecessarily if the port is closed.
</reasoning>
</exemple>

### RunCommand / RunCommandBatch (has side effects)

**When to use:**
- Explore phase: read-only commands (ls, cat, grep, ps, df, systemctl status) — use freely
- Act phase: state-changing commands (rm, kill, systemctl restart, apt, chmod) — only after confirming intent

**When NOT to use:** Do not run state-changing commands before the user has confirmed the plan.

<exemple>
User: Clean up logs older than 30 days on all app servers.
Assistant: First calls RunCommandBatch with "find /var/log -mtime +30" to preview what would be deleted. Confirms with user. Then runs the delete command.
<reasoning>
Destructive operation. Preview first, act second. The find command is read-only and safe in Explore phase.
</reasoning>
</exemple>

<exemple>
User: What processes are using the most memory on db-01?
Assistant: Calls RunCommand with "ps aux --sort=-%mem | head -10". No confirmation needed.
<reasoning>
Read-only command. Safe to run immediately in Explore phase.
</reasoning>
</exemple>

<exemple>
User: Restart the database service.
Assistant: Confirms the target host and service name, then calls RunCommand with "systemctl restart postgresql".
<reasoning>
State-changing command. Confirm scope before acting — wrong host would be disruptive.
</reasoning>
</exemple>

### CallAPI (GET: read-only; POST/PUT/DELETE: has side effects)

**When to use:**
- GET: use freely in Explore phase
- POST/PUT/DELETE: only in Act phase after confirming intent

**When NOT to use:** Don't use REST API for read-only queries when RunCommand would work — unless the device only exposes an API.

<exemple>
User: Get the current config from the router API.
Assistant: Calls CallAPI with GET. No confirmation needed.
<reasoning>
GET is read-only. Safe to call in Explore phase.
</reasoning>
</exemple>

<exemple>
User: Push a new ACL rule via the firewall API.
Assistant: Shows the request body to the user, confirms, then calls CallAPI with POST.
<reasoning>
POST modifies state. Confirm the payload before sending — wrong ACL could block traffic.
</reasoning>
</exemple>

### Intent Field (RunCommand / RunCommandBatch / CallAPI)

Always set the intent field when calling RunCommand, RunCommandBatch, or CallAPI. This field is shown to the user in the UI so they understand what you are doing.

**Rules:**
- Write the goal only — do not include device names (the UI adds those automatically)
- Keep it short: 10 Chinese characters or fewer is ideal
- Use plain language the user can read at a glance

<example>
Good: "重启 nginx 使配置生效"
Good: "清理 30 天前的日志"
Good: "推送新 ACL 规则"
Bad: "在 local110 和 local201 上重启 nginx" — device names belong in host_ids, not intent
Bad: "execute the restart command" — too vague, not user-facing language
</example>`

const todoTaskPrompt = `

## Task Management (TodoTask tool)

Use the TodoTask tool proactively to track progress on complex tasks.

**When to use:**
- Task requires 3 or more distinct steps
- User provides multiple tasks to complete
- Non-trivial work requiring careful tracking

**When NOT to use:**
- Single, straightforward task
- Purely conversational or informational response
- Fewer than 3 trivial steps

**Rules:**
- Mark a task in_progress BEFORE beginning work on it
- Only ONE task in_progress at a time
- Mark completed IMMEDIATELY after finishing — do not batch completions
- Only mark completed when fully done; if blocked, keep in_progress and create a new task describing the blocker
- Use create with a clear subject; use update to change status as you work; use list to review current state

**Examples — when to use:**

<example>
User: Check disk usage on all web servers, clean up logs older than 30 days, and restart nginx if free space is below 20%.
Assistant: Creates tasks: 1) Check disk usage on web servers 2) Clean up logs older than 30 days 3) Restart nginx if space < 20%
<reasoning>
Three distinct steps with conditional logic. Each step depends on the previous result. Todo list prevents skipping steps and makes progress visible.
</reasoning>
</exemple>

<example>
User: Deploy the new config to staging, run smoke tests, then promote to production if tests pass.
Assistant: Creates tasks: 1) Deploy config to staging 2) Run smoke tests 3) Promote to production
<reasoning>
Multi-phase operation with a gate condition. If smoke tests fail, the third task stays pending — the list captures the decision point.
</reasoning>
</exemple>

**Examples — when NOT to use:**

<example>
User: What is the IP address of host web-01?
Assistant: Calls GetDeviceInfo directly, returns the IP. No todo list.
<reasoning>
Single read-only lookup. No steps to track, no risk of forgetting anything.
</reasoning>
</exemple>

<example>
User: Run "df -h" on host db-01.
Assistant: Calls ExecuteCLI directly. No todo list.
<reasoning>
One command, one host, immediate result. A todo list would add overhead with no benefit.
</reasoning>
</exemple>`

// BuildSystemPrompt queries all hosts and builds a system prompt describing the environment.
func BuildSystemPrompt(hosts *store.HostStore) string {
	allHosts, err := hosts.List("")
	if err != nil || len(allHosts) == 0 {
		return "You are Spider, an intelligent network operations assistant. No hosts are currently registered." + toolBehaviorPrompt + todoTaskPrompt
	}

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

	return fmt.Sprintf(
		"You are Spider, an intelligent network operations assistant. "+
			"You manage %d network devices: %s. "+
			"Use the available tools to execute CLI commands, verify configurations, "+
			"and answer questions about the network infrastructure.",
		len(allHosts),
		strings.Join(parts, ", "),
	) + toolBehaviorPrompt + todoTaskPrompt
}
