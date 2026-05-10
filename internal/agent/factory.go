package agent

import (
	"fmt"
	"strings"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/llm"
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
func (f *Factory) NewAgent(systemPrompt string) *Agent {
	registry := NewToolRegistry()
	registry.Register(NewListDevicesTool(f.Hosts))
	registry.Register(NewGetDeviceInfoTool(f.Hosts))
	registry.Register(NewExecuteCLITool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewBatchExecuteTool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewVerifyTool(f.Hosts, f.AccessFaces, f.SSHPool, f.SSHKeys))
	registry.Register(NewCallRESTAPITool(f.AccessFaces))

	hooks := NewHookChain()
	if f.Enforcer != nil {
		hooks.AddBefore(PermissionHook(f.Enforcer, f.PermissionMode))
	} else {
		hooks.AddBefore(DefaultRiskHook())
	}

	compactor := NewCompactor(f.LLMClient, f.SummaryStore, f.MsgStore, f.LLMModel, f.CompactionCfg)
	return NewAgent(AgentConfig{
		LLMClient:    f.LLMClient,
		Registry:     registry,
		Hooks:        hooks,
		MsgStore:     f.MsgStore,
		SystemPrompt: systemPrompt,
		MaxTurns:     15,
		Compactor:    compactor,
	})
}

// BuildSystemPrompt queries all hosts and builds a system prompt describing the environment.
func BuildSystemPrompt(hosts *store.HostStore) string {
	allHosts, err := hosts.List("")
	if err != nil || len(allHosts) == 0 {
		return "You are Spider, an intelligent network operations assistant. No hosts are currently registered."
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
	)
}
