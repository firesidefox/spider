package chatruntime

import (
	"context"
	"testing"
	"time"

	"github.com/spiderai/spider/internal/agent"
)

func TestChatWaiterLifecycle(t *testing.T) {
	rt := New()
	waiter := agent.NewConfirmationWaiter()

	rt.StoreChatWaiter("conv-1", waiter)
	if got := rt.GetChatWaiter("conv-1"); got != waiter {
		t.Fatalf("expected stored waiter, got %#v", got)
	}

	rt.RemoveChatWaiter("conv-1")
	if got := rt.GetChatWaiter("conv-1"); got != nil {
		t.Fatalf("expected waiter removal, got %#v", got)
	}
}

func TestConversationCancelLifecycle(t *testing.T) {
	rt := New()
	called := make(chan struct{})
	cancel := func() { close(called) }

	rt.StoreConvCancel("conv-1", cancel)
	if !rt.CancelConv("conv-1") {
		t.Fatal("expected cancel to return true")
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("stored cancel was not called")
	}
	if rt.CancelConv("conv-1") {
		t.Fatal("expected second cancel to return false after removal")
	}
}

func TestTryClaimInjectConsumeAndRelease(t *testing.T) {
	rt := New()

	ch, ok := rt.TryClaimConv("conv-1")
	if !ok {
		t.Fatal("expected first claim to succeed")
	}
	if _, ok := rt.TryClaimConv("conv-1"); ok {
		t.Fatal("expected second claim to fail while conversation is running")
	}

	if queued, full := rt.TryInject("conv-1", "one"); !queued || full {
		t.Fatalf("expected first inject queued=true full=false, got queued=%v full=%v", queued, full)
	}
	if queued, full := rt.TryInject("conv-1", "two\n\nwith break"); !queued || full {
		t.Fatalf("expected second inject queued=true full=false, got queued=%v full=%v", queued, full)
	}
	if queued, full := rt.TryInject("conv-1", "three"); !queued || full {
		t.Fatalf("expected third inject queued=true full=false, got queued=%v full=%v", queued, full)
	}

	if got := rt.GetQueuedMsgs("conv-1"); len(got) != 3 || got[0] != "one" || got[1] != "two\n\nwith break" || got[2] != "three" {
		t.Fatalf("unexpected queued messages: %#v", got)
	}

	rt.ConsumeQueuedMsgs("conv-1", 2)
	if got := rt.GetQueuedMsgs("conv-1"); len(got) != 1 || got[0] != "three" {
		t.Fatalf("expected only third message after count-based consume, got %#v", got)
	}

	rt.ReleaseConv("conv-1")
	if queued, full := rt.TryInject("conv-1", "after-release"); queued || full {
		t.Fatalf("expected inject after release to miss running conversation, got queued=%v full=%v", queued, full)
	}
	if got := rt.GetQueuedMsgs("conv-1"); got != nil {
		t.Fatalf("expected release to clear queue, got %#v", got)
	}
	// Drain any remaining messages from the channel
	for {
		select {
		case _, open := <-ch:
			if !open {
				return // Channel is closed, test passes
			}
		case <-time.After(time.Second):
			t.Fatal("expected released injection channel to close")
		}
	}
}

func TestTryInjectFull(t *testing.T) {
	rt := New()
	if _, ok := rt.TryClaimConv("conv-1"); !ok {
		t.Fatal("expected claim to succeed")
	}

	for i := range 32 {
		if queued, full := rt.TryInject("conv-1", "msg"); !queued || full {
			t.Fatalf("inject %d expected queued=true full=false, got queued=%v full=%v", i, queued, full)
		}
	}
	if queued, full := rt.TryInject("conv-1", "overflow"); queued || !full {
		t.Fatalf("expected overflow to return queued=false full=true, got queued=%v full=%v", queued, full)
	}
}

func TestRegisterSSEClientAndDrain(t *testing.T) {
	rt := New()
	rt.BufferSSEEvent("conv-1", []byte("old-1"))
	rt.BufferSSEEvent("conv-1", []byte("old-2"))

	ch := make(chan []byte, 2)
	buffered := rt.RegisterSSEClientAndDrain("conv-1", ch)
	if len(buffered) != 2 || string(buffered[0]) != "old-1" || string(buffered[1]) != "old-2" {
		t.Fatalf("unexpected drained buffer: %#v", buffered)
	}

	buffered = rt.RegisterSSEClientAndDrain("conv-1", make(chan []byte, 1))
	if len(buffered) != 0 {
		t.Fatalf("expected second drain to be empty, got %#v", buffered)
	}
}

func TestBufferAndBroadcastSSE(t *testing.T) {
	rt := New()
	ch := make(chan []byte, 1)
	buffered := rt.RegisterSSEClientAndDrain("conv-1", ch)
	if len(buffered) != 0 {
		t.Fatalf("expected empty initial buffer, got %#v", buffered)
	}

	rt.BufferAndBroadcastSSE("conv-1", []byte("live-1"))
	select {
	case got := <-ch:
		if string(got) != "live-1" {
			t.Fatalf("unexpected live event: %q", string(got))
		}
	case <-time.After(time.Second):
		t.Fatal("expected live event")
	}

	reconnect := make(chan []byte, 1)
	buffered = rt.RegisterSSEClientAndDrain("conv-1", reconnect)
	if len(buffered) != 1 || string(buffered[0]) != "live-1" {
		t.Fatalf("expected buffered live event for reconnect, got %#v", buffered)
	}
}

func TestBufferSSEEventDoesNotBroadcast(t *testing.T) {
	rt := New()
	ch := make(chan []byte, 1)
	rt.RegisterSSEClientAndDrain("conv-1", ch)

	rt.BufferSSEEvent("conv-1", []byte("buf-only"))
	select {
	case got := <-ch:
		t.Fatalf("BufferSSEEvent must not broadcast to clients, got %q", string(got))
	default:
	}
}

func TestBroadcastSSEDoesNotBuffer(t *testing.T) {
	rt := New()
	ch := make(chan []byte, 1)
	rt.RegisterSSEClientAndDrain("conv-1", ch)

	rt.BroadcastSSE("conv-1", []byte("live"))
	<-ch // drain the live event

	reconnect := make(chan []byte, 1)
	buffered := rt.RegisterSSEClientAndDrain("conv-1", reconnect)
	if len(buffered) != 0 {
		t.Fatalf("BroadcastSSE must not write to reconnect buffer, got %#v", buffered)
	}
}

func TestUnregisterSSEClient(t *testing.T) {
	rt := New()
	ch := make(chan []byte, 1)
	rt.RegisterSSEClientAndDrain("conv-1", ch)
	rt.UnregisterSSEClient("conv-1", ch)

	rt.BroadcastSSE("conv-1", []byte("ignored"))
	select {
	case got := <-ch:
		t.Fatalf("unregistered client received event %q", string(got))
	default:
	}
}

func TestGlobalSSEClients(t *testing.T) {
	rt := New()
	ch := make(chan []byte, 1)

	rt.AddGlobalSSEClient(ch)
	rt.BroadcastGlobalSSE([]byte("global-1"))
	select {
	case got := <-ch:
		if string(got) != "global-1" {
			t.Fatalf("unexpected global event: %q", string(got))
		}
	case <-time.After(time.Second):
		t.Fatal("expected global event")
	}

	rt.RemoveGlobalSSEClient(ch)
	rt.BroadcastGlobalSSE([]byte("global-2"))
	select {
	case got := <-ch:
		t.Fatalf("removed global client received event %q", string(got))
	default:
	}
}

func TestCancelRemoveWithoutCalling(t *testing.T) {
	rt := New()
	ctx, cancel := context.WithCancel(context.Background())
	rt.StoreConvCancel("conv-1", cancel)
	rt.RemoveConvCancel("conv-1")

	select {
	case <-ctx.Done():
		t.Fatal("RemoveConvCancel must not call cancel")
	default:
	}
	if rt.CancelConv("conv-1") {
		t.Fatal("expected removed cancel to be absent")
	}
}

func TestSSEBufferCapEviction(t *testing.T) {
	rt := New()
	for i := range 501 {
		rt.BufferSSEEvent("conv-1", []byte{byte(i % 256)})
	}
	ch := make(chan []byte, 501)
	buffered := rt.RegisterSSEClientAndDrain("conv-1", ch)
	if len(buffered) != 500 {
		t.Fatalf("expected 500 buffered events after cap eviction, got %d", len(buffered))
	}
	// oldest event (i=0) should be evicted; first retained is i=1
	if buffered[0][0] != 1 {
		t.Fatalf("expected oldest event evicted, first retained byte=1, got %d", buffered[0][0])
	}
}

func TestClearSSEBuffer(t *testing.T) {
	rt := New()
	rt.BufferSSEEvent("conv-1", []byte("keep"))
	rt.ClearSSEBuffer("conv-1")

	ch := make(chan []byte, 1)
	buffered := rt.RegisterSSEClientAndDrain("conv-1", ch)
	if len(buffered) != 0 {
		t.Fatalf("expected empty buffer after ClearSSEBuffer, got %#v", buffered)
	}
}
