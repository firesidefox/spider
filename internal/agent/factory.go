package agent

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/rag"
	"github.com/spiderai/spider/internal/ssh"
	"github.com/spiderai/spider/internal/store"
)

// Factory holds shared dependencies for creating Agent instances.
type Factory struct {
	LLMClient llm.Client
	RAGStore  *rag.Store
	Hosts     *store.HostStore
	SSHPool   *ssh.Pool
	SSHKeys   *store.SSHKeyStore
	Logs      *store.LogStore
	MsgStore  MessageStorer
	cfg       *config.Config
	db        *sql.DB
	docStore  *store.DocumentStore
}

// NewFactory creates a Factory.
// NOTE: This is a stub — Task 6 will rewrite it to read providers from the DB.
// Returns an error until the ProviderStore is wired in.
func NewFactory(
	cfg *config.Config,
	database *sql.DB,
	hosts *store.HostStore,
	pool *ssh.Pool,
	keys *store.SSHKeyStore,
	logs *store.LogStore,
	msgs MessageStorer,
	docs *store.DocumentStore,
) (*Factory, error) {
	return nil, fmt.Errorf("factory requires ProviderStore (not yet wired)")
}

// NewAgent creates a new Agent with all tools registered.
func (f *Factory) NewAgent(systemPrompt string) *Agent {
	registry := NewToolRegistry()
	registry.Register(NewGetDeviceInfoTool(f.Hosts))
	registry.Register(NewExecuteCLITool(f.Hosts, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewBatchExecuteTool(f.Hosts, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewVerifyTool(f.Hosts, f.SSHPool, f.SSHKeys))
	registry.Register(NewCallRESTAPITool())
	if f.RAGStore != nil {
		registry.Register(NewSearchDocsTool(f.RAGStore))
	}

	hooks := NewHookChain()
	hooks.AddBefore(DefaultRiskHook())

	return NewAgent(AgentConfig{
		LLMClient:    f.LLMClient,
		Registry:     registry,
		Hooks:        hooks,
		MsgStore:     f.MsgStore,
		SystemPrompt: systemPrompt,
		MaxTurns:     15,
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
