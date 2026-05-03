package permission

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ApprovalRequest represents a pending approval request.
type ApprovalRequest struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	Command     string    `json:"command"`
	Host        string    `json:"host"`
	RiskLevel   RiskLevel `json:"risk_level"`
	RiskReason  string    `json:"risk_reason"`
	RequestedAt time.Time `json:"requested_at"`
}

type ApprovalResult struct {
	Approved   bool   `json:"approved"`
	ApprovedBy string `json:"approved_by"`
}

// ApprovalManager manages approval requests in memory.
// Thread-safe for concurrent use.
type ApprovalManager struct {
	mu          sync.RWMutex
	pending     map[string]*ApprovalRequest
	results     map[string]*resultEntry
	waiters     map[string]chan *ApprovalResult
	subscribers []chan *ApprovalRequest
}

type resultEntry struct {
	result    *ApprovalResult
	resolvedAt time.Time
}

// NewApprovalManager creates a new ApprovalManager.
func NewApprovalManager() *ApprovalManager {
	return &ApprovalManager{
		pending: make(map[string]*ApprovalRequest),
		results: make(map[string]*resultEntry),
		waiters: make(map[string]chan *ApprovalResult),
	}
}

// Create creates a new approval request and returns it.
// Notifies all subscribers.
func (m *ApprovalManager) Create(sessionID, command, host string, level RiskLevel, reason string) *ApprovalRequest {
	req := &ApprovalRequest{
		ID:          uuid.New().String(),
		SessionID:   sessionID,
		Command:     command,
		Host:        host,
		RiskLevel:   level,
		RiskReason:  reason,
		RequestedAt: time.Now(),
	}

	m.mu.Lock()
	m.pending[req.ID] = req
	subs := append([]chan *ApprovalRequest(nil), m.subscribers...)
	m.mu.Unlock()

	// notify subscribers (non-blocking)
	for _, ch := range subs {
		select {
		case ch <- req:
		default:
			log.Printf("WARNING: approval subscriber channel full, event dropped for request %s", req.ID)
		}
	}

	return req
}

// Wait blocks until the approval request is responded to or context is canceled.
func (m *ApprovalManager) Wait(ctx context.Context, id string) (*ApprovalResult, error) {
	m.mu.Lock()
	if entry, ok := m.results[id]; ok {
		m.mu.Unlock()
		return entry.result, nil
	}
	// create waiter channel
	ch := make(chan *ApprovalResult, 1)
	m.waiters[id] = ch
	m.mu.Unlock()

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		m.mu.Lock()
		delete(m.waiters, id)
		m.mu.Unlock()
		return nil, ctx.Err()
	}
}

// Respond records the approval decision and wakes up any waiting goroutine.
func (m *ApprovalManager) Respond(id string, approved bool, approvedBy string) {
	result := &ApprovalResult{
		Approved:   approved,
		ApprovedBy: approvedBy,
	}

	m.mu.Lock()
	m.results[id] = &resultEntry{result: result, resolvedAt: time.Now()}
	delete(m.pending, id)
	if ch, ok := m.waiters[id]; ok {
		delete(m.waiters, id)
		m.mu.Unlock()
		ch <- result
		return
	}
	m.mu.Unlock()
}

// Pending returns all pending approval requests.
func (m *ApprovalManager) Pending() []*ApprovalRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ApprovalRequest, 0, len(m.pending))
	for _, req := range m.pending {
		result = append(result, req)
	}
	return result
}

// Subscribe returns a channel that receives new approval requests.
// Caller must call Unsubscribe when done.
func (m *ApprovalManager) Subscribe() chan *ApprovalRequest {
	ch := make(chan *ApprovalRequest, 10)
	m.mu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscription channel.
func (m *ApprovalManager) Unsubscribe(ch chan *ApprovalRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, sub := range m.subscribers {
		if sub == ch {
			m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// CleanupExpired removes resolved approval results older than maxAge.
func (m *ApprovalManager) CleanupExpired(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for id, entry := range m.results {
		if entry.resolvedAt.Before(cutoff) {
			delete(m.results, id)
			count++
		}
	}
	return count
}
