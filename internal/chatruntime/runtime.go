package chatruntime

import (
	"context"
	"sync"

	"github.com/spiderai/spider/internal/agent"
)

const maxSSEBufferEvents = 500

type Runtime struct {
	chatWaiters   map[string]*agent.ConfirmationWaiter
	chatWaitersMu sync.Mutex

	convCancels   map[string]context.CancelFunc
	convCancelsMu sync.Mutex

	convInjectChs   map[string]chan string
	convQueuedMsgs  map[string][]string
	convInjectChsMu sync.Mutex

	sseClients   map[string][]chan []byte
	sseClientsMu sync.Mutex

	sseBuffers   map[string][][]byte
	sseBuffersMu sync.Mutex

	globalSSEClients   []chan []byte
	globalSSEClientsMu sync.Mutex
}

func New() *Runtime {
	return &Runtime{}
}

func (r *Runtime) StoreChatWaiter(convID string, w *agent.ConfirmationWaiter) {
	r.chatWaitersMu.Lock()
	defer r.chatWaitersMu.Unlock()
	if r.chatWaiters == nil {
		r.chatWaiters = make(map[string]*agent.ConfirmationWaiter)
	}
	r.chatWaiters[convID] = w
}

func (r *Runtime) GetChatWaiter(convID string) *agent.ConfirmationWaiter {
	r.chatWaitersMu.Lock()
	defer r.chatWaitersMu.Unlock()
	return r.chatWaiters[convID]
}

func (r *Runtime) RemoveChatWaiter(convID string) {
	r.chatWaitersMu.Lock()
	defer r.chatWaitersMu.Unlock()
	delete(r.chatWaiters, convID)
}

func (r *Runtime) StoreConvCancel(convID string, cancel context.CancelFunc) {
	r.convCancelsMu.Lock()
	defer r.convCancelsMu.Unlock()
	if r.convCancels == nil {
		r.convCancels = make(map[string]context.CancelFunc)
	}
	r.convCancels[convID] = cancel
}

func (r *Runtime) CancelConv(convID string) bool {
	r.convCancelsMu.Lock()
	defer r.convCancelsMu.Unlock()
	cancel, ok := r.convCancels[convID]
	if ok {
		cancel()
		delete(r.convCancels, convID)
	}
	return ok
}

func (r *Runtime) RemoveConvCancel(convID string) {
	r.convCancelsMu.Lock()
	defer r.convCancelsMu.Unlock()
	delete(r.convCancels, convID)
}

func (r *Runtime) TryClaimConv(convID string) (chan string, bool) {
	r.convInjectChsMu.Lock()
	defer r.convInjectChsMu.Unlock()
	if r.convInjectChs == nil {
		r.convInjectChs = make(map[string]chan string)
	}
	if _, running := r.convInjectChs[convID]; running {
		return nil, false
	}
	ch := make(chan string, 32)
	r.convInjectChs[convID] = ch
	return ch, true
}

func (r *Runtime) TryInject(convID, msg string) (queued bool, full bool) {
	r.convInjectChsMu.Lock()
	defer r.convInjectChsMu.Unlock()
	ch, ok := r.convInjectChs[convID]
	if !ok {
		return false, false
	}
	select {
	case ch <- msg:
		if r.convQueuedMsgs == nil {
			r.convQueuedMsgs = make(map[string][]string)
		}
		r.convQueuedMsgs[convID] = append(r.convQueuedMsgs[convID], msg)
		return true, false
	default:
		return false, true
	}
}

func (r *Runtime) ConsumeQueuedMsgs(convID string, n int) {
	if n <= 0 {
		return
	}
	r.convInjectChsMu.Lock()
	defer r.convInjectChsMu.Unlock()
	queue := r.convQueuedMsgs[convID]
	if len(queue) == 0 {
		return
	}
	if n >= len(queue) {
		delete(r.convQueuedMsgs, convID)
	} else {
		r.convQueuedMsgs[convID] = queue[n:]
	}
}

func (r *Runtime) GetQueuedMsgs(convID string) []string {
	r.convInjectChsMu.Lock()
	defer r.convInjectChsMu.Unlock()
	q := r.convQueuedMsgs[convID]
	if len(q) == 0 {
		return nil
	}
	out := make([]string, len(q))
	copy(out, q)
	return out
}

func (r *Runtime) ReleaseConv(convID string) {
	r.convInjectChsMu.Lock()
	defer r.convInjectChsMu.Unlock()
	if ch, ok := r.convInjectChs[convID]; ok {
		delete(r.convInjectChs, convID)
		// TryInject is non-blocking; after delete, no new sends can race with close.
		close(ch)
	}
	delete(r.convQueuedMsgs, convID)
}

func (r *Runtime) UnregisterSSEClient(convID string, ch chan []byte) {
	r.sseClientsMu.Lock()
	defer r.sseClientsMu.Unlock()
	clients := r.sseClients[convID]
	for i, c := range clients {
		if c == ch {
			r.sseClients[convID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	if len(r.sseClients[convID]) == 0 {
		delete(r.sseClients, convID)
	}
}

func (r *Runtime) BroadcastSSE(convID string, data []byte) {
	r.sseClientsMu.Lock()
	defer r.sseClientsMu.Unlock()
	for _, ch := range r.sseClients[convID] {
		select {
		case ch <- data:
		default:
		}
	}
}

func (r *Runtime) BufferAndBroadcastSSE(convID string, data []byte) {
	r.sseBuffersMu.Lock()
	defer r.sseBuffersMu.Unlock()
	r.sseClientsMu.Lock()
	defer r.sseClientsMu.Unlock()
	if r.sseBuffers == nil {
		r.sseBuffers = make(map[string][][]byte)
	}
	buf := r.sseBuffers[convID]
	if len(buf) >= maxSSEBufferEvents {
		r.sseBuffers[convID] = append(buf[1:], data)
	} else {
		r.sseBuffers[convID] = append(buf, data)
	}
	for _, ch := range r.sseClients[convID] {
		select {
		case ch <- data:
		default:
		}
	}
}

func (r *Runtime) BufferSSEEvent(convID string, data []byte) {
	r.sseBuffersMu.Lock()
	defer r.sseBuffersMu.Unlock()
	if r.sseBuffers == nil {
		r.sseBuffers = make(map[string][][]byte)
	}
	buf := r.sseBuffers[convID]
	if len(buf) >= maxSSEBufferEvents {
		r.sseBuffers[convID] = append(buf[1:], data)
		return
	}
	r.sseBuffers[convID] = append(buf, data)
}

func (r *Runtime) RegisterSSEClientAndDrain(convID string, ch chan []byte) [][]byte {
	r.sseBuffersMu.Lock()
	defer r.sseBuffersMu.Unlock()
	r.sseClientsMu.Lock()
	defer r.sseClientsMu.Unlock()
	if r.sseClients == nil {
		r.sseClients = make(map[string][]chan []byte)
	}
	r.sseClients[convID] = append(r.sseClients[convID], ch)
	src := r.sseBuffers[convID]
	if len(src) == 0 {
		return nil
	}
	buf := make([][]byte, len(src))
	copy(buf, src)
	return buf
}

func (r *Runtime) ClearSSEBuffer(convID string) {
	r.sseBuffersMu.Lock()
	defer r.sseBuffersMu.Unlock()
	delete(r.sseBuffers, convID)
}

func (r *Runtime) AddGlobalSSEClient(ch chan []byte) {
	r.globalSSEClientsMu.Lock()
	defer r.globalSSEClientsMu.Unlock()
	r.globalSSEClients = append(r.globalSSEClients, ch)
}

func (r *Runtime) RemoveGlobalSSEClient(ch chan []byte) {
	r.globalSSEClientsMu.Lock()
	defer r.globalSSEClientsMu.Unlock()
	clients := make([]chan []byte, 0, len(r.globalSSEClients))
	for _, c := range r.globalSSEClients {
		if c != ch {
			clients = append(clients, c)
		}
	}
	r.globalSSEClients = clients
}

func (r *Runtime) BroadcastGlobalSSE(data []byte) {
	r.globalSSEClientsMu.Lock()
	defer r.globalSSEClientsMu.Unlock()
	for _, ch := range r.globalSSEClients {
		select {
		case ch <- data:
		default:
		}
	}
}
