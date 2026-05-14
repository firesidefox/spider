package monitor

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

const (
	defaultProbePort     = 22
	defaultProbeInterval = 2 * time.Second
	maxConcurrency       = 20
)

// Status represents host liveness.
type Status int

const (
	StatusUnknown Status = iota
	StatusUp
	StatusDown
)

// OnChangeFunc is called when a host's status changes.
type OnChangeFunc func(host *models.Host, status Status)

// Monitor probes hosts via TCP and fires onChange on status transitions.
type Monitor struct {
	hostStore *store.HostStore
	faceStore *store.AccessFaceStore
	onChange  OnChangeFunc
	tag       string

	mu      sync.Mutex
	statuses map[string]Status // keyed by host ID
	stops    map[string]chan struct{}
}

// New creates a Monitor. tag filters which hosts to watch ("" = all).
func New(hs *store.HostStore, fs *store.AccessFaceStore, tag string, onChange OnChangeFunc) *Monitor {
	return &Monitor{
		hostStore: hs,
		faceStore: fs,
		onChange:  onChange,
		tag:       tag,
		statuses:  make(map[string]Status),
		stops:     make(map[string]chan struct{}),
	}
}

// Start loads hosts and begins probing. Blocks until Stop is called.
func (m *Monitor) Start() error {
	hosts, err := m.hostStore.List(m.tag)
	if err != nil {
		return fmt.Errorf("monitor: list hosts: %w", err)
	}

	sem := make(chan struct{}, maxConcurrency)

	for _, h := range hosts {
		h := h
		face, err := m.primaryFace(h.ID)
		if err != nil {
			continue
		}

		port := face.ProbePort
		if port == 0 {
			port = defaultProbePort
		}
		interval := time.Duration(face.ProbeInterval) * time.Second
		if interval <= 0 {
			interval = defaultProbeInterval
		}

		stop := make(chan struct{})
		m.mu.Lock()
		m.stops[h.ID] = stop
		m.mu.Unlock()

		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			m.probeLoop(h, port, interval, stop)
		}()
	}

	// Wait until all stop channels are closed (Stop() closes them).
	<-make(chan struct{})
	return nil
}

// Stop halts all probe goroutines.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, ch := range m.stops {
		close(ch)
		delete(m.stops, id)
	}
}

// probeLoop dials host:port on each interval tick until stop is closed.
func (m *Monitor) probeLoop(h *models.Host, port int, interval time.Duration, stop <-chan struct{}) {
	addr := fmt.Sprintf("%s:%d", h.IP, port)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			m.probe(h, addr)
		}
	}
}

func (m *Monitor) probe(h *models.Host, addr string) {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	var next Status
	if err != nil {
		next = StatusDown
	} else {
		conn.Close()
		next = StatusUp
	}

	m.mu.Lock()
	prev := m.statuses[h.ID]
	if prev == next {
		m.mu.Unlock()
		return
	}
	m.statuses[h.ID] = next
	m.mu.Unlock()

	if m.onChange != nil {
		m.onChange(h, next)
	}
}

// primaryFace returns the first AccessFace for the host (any type).
func (m *Monitor) primaryFace(hostID string) (*models.AccessFace, error) {
	faces, err := m.faceStore.ListByHost(hostID)
	if err != nil {
		return nil, err
	}
	if len(faces) == 0 {
		return nil, fmt.Errorf("no access face for host %s", hostID)
	}
	return faces[0], nil
}
