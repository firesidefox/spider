package monitor

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

const (
	defaultProbeInterval = 2 * time.Second
	defaultProbePort     = 22
	dialTimeout          = 2 * time.Second
	maxConcurrent        = 20
)

type Monitor struct {
	hostStore *store.HostStore
	faceStore *store.AccessFaceStore
	onChange  func(hostID string, online bool)

	statuses   map[string]bool
	lastProbed map[string]time.Time
	mu         sync.RWMutex

	cancel context.CancelFunc
}

func New(
	hostStore *store.HostStore,
	faceStore *store.AccessFaceStore,
	onChange func(hostID string, online bool),
) *Monitor {
	return &Monitor{
		hostStore:  hostStore,
		faceStore:  faceStore,
		onChange:   onChange,
		statuses:   make(map[string]bool),
		lastProbed: make(map[string]time.Time),
	}
}

func (m *Monitor) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	go m.loop(ctx)
}

func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// GetStatus returns the last known status for a host. Returns true (online) if unknown.
func (m *Monitor) GetStatus(hostID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.statuses[hostID]
	if !ok {
		return true
	}
	return v
}

// Statuses returns a snapshot of all known statuses.
func (m *Monitor) Statuses() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]bool, len(m.statuses))
	for k, v := range m.statuses {
		out[k] = v
	}
	return out
}

func (m *Monitor) loop(ctx context.Context) {
	ticker := time.NewTicker(defaultProbeInterval)
	defer ticker.Stop()
	m.probeAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.probeAll(ctx)
		}
	}
}

func (m *Monitor) probeAll(ctx context.Context) {
	hosts, err := m.hostStore.List("")
	if err != nil {
		return
	}

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, h := range hosts {
		h := h
		interval, port := m.probeConfig(h.ID)

		m.mu.RLock()
		last := m.lastProbed[h.ID]
		m.mu.RUnlock()

		if time.Since(last) < interval {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			m.probeOne(h, port)
		}()
	}
	wg.Wait()
}

func (m *Monitor) probeOne(h *models.Host, port int) {
	addr := net.JoinHostPort(h.IP, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	online := err == nil
	if conn != nil {
		conn.Close()
	}

	m.mu.Lock()
	prev, known := m.statuses[h.ID]
	m.statuses[h.ID] = online
	m.lastProbed[h.ID] = time.Now()
	m.mu.Unlock()

	if !known || prev != online {
		m.onChange(h.ID, online)
	}
}

func (m *Monitor) probeConfig(hostID string) (time.Duration, int) {
	faces, err := m.faceStore.ListByHost(hostID)
	if err != nil || len(faces) == 0 {
		return defaultProbeInterval, defaultProbePort
	}
	for _, f := range faces {
		if f.Type == models.FaceSSH {
			interval := defaultProbeInterval
			port := defaultProbePort
			if f.ProbeInterval > 0 {
				interval = time.Duration(f.ProbeInterval) * time.Second
			}
			if f.ProbePort > 0 {
				port = f.ProbePort
			}
			return interval, port
		}
	}
	return defaultProbeInterval, defaultProbePort
}
