package ssh

import (
	"fmt"
	"sync"
	"time"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type poolEntry struct {
	client    *Client
	lastUsed  time.Time
	inUse     bool
}

// Pool 是 SSH 连接池，按 hostID 缓存连接。
type Pool struct {
	mu      sync.Mutex
	entries map[string]*poolEntry
	ttl     time.Duration
}

// NewPool 创建一个新的连接池。
func NewPool(ttl time.Duration) *Pool {
	p := &Pool{
		entries: make(map[string]*poolEntry),
		ttl:     ttl,
	}
	return p
}

// Get 从池中获取连接，不存在或已过期则新建。
func (p *Pool) Get(host *models.Host, hs *store.HostStore) (*Client, error) {
	p.mu.Lock()
	entry, ok := p.entries[host.ID]
	if ok && !entry.inUse && time.Since(entry.lastUsed) < p.ttl {
		entry.inUse = true
		p.mu.Unlock()
		return entry.client, nil
	}
	p.mu.Unlock()

	// 新建连接
	client, err := NewClient(host, hs)
	if err != nil {
		return nil, fmt.Errorf("创建 SSH 连接失败: %w", err)
	}

	p.mu.Lock()
	p.entries[host.ID] = &poolEntry{
		client:   client,
		lastUsed: time.Now(),
		inUse:    true,
	}
	p.mu.Unlock()
	return client, nil
}

// Release 将连接归还连接池。
func (p *Pool) Release(hostID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if entry, ok := p.entries[hostID]; ok {
		entry.inUse = false
		entry.lastUsed = time.Now()
	}
}

// StartCleanup 启动后台 goroutine 定期清理过期连接。
func (p *Pool) StartCleanup() {
	go func() {
		ticker := time.NewTicker(p.ttl / 2)
		defer ticker.Stop()
		for range ticker.C {
			p.cleanup()
		}
	}()
}

func (p *Pool) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, entry := range p.entries {
		if !entry.inUse && time.Since(entry.lastUsed) >= p.ttl {
			entry.client.Close()
			delete(p.entries, id)
		}
	}
}

// Close 关闭所有连接。
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, entry := range p.entries {
		entry.client.Close()
		delete(p.entries, id)
	}
}
