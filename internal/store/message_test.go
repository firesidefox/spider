package store

import "testing"

func TestMessageStore_SaveAndList(t *testing.T) {
	database := setupTestDB(t)
	cs := NewConversationStore(database)
	ms := NewMessageStore(database)

	conv, _ := cs.Create("user-1", "")

	err := ms.Save(conv.ID, "user", `{"type":"text","text":"hello"}`, "")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	err = ms.Save(conv.ID, "assistant", `{"type":"text","text":"hi"}`, `[{"id":"t1","name":"RunCommand","duration_ms":100}]`)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	msgs, err := ms.ListByConversation(conv.ID)
	if err != nil {
		t.Fatalf("ListByConversation: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("msgs[0].Role = %q, want user", msgs[0].Role)
	}
	if msgs[1].ToolCalls == "" {
		t.Error("msgs[1].ToolCalls should not be empty")
	}
}

func TestMessageStore_DeleteByConversation(t *testing.T) {
	database := setupTestDB(t)
	cs := NewConversationStore(database)
	ms := NewMessageStore(database)

	conv, _ := cs.Create("user-1", "")
	ms.Save(conv.ID, "user", `{"text":"hello"}`, "")

	err := ms.DeleteByConversation(conv.ID)
	if err != nil {
		t.Fatalf("DeleteByConversation: %v", err)
	}
	msgs, _ := ms.ListByConversation(conv.ID)
	if len(msgs) != 0 {
		t.Errorf("len = %d, want 0", len(msgs))
	}
}

func TestListAfterMessage_EmptyID(t *testing.T) {
	database := setupTestDB(t)
	cs := NewConversationStore(database)
	ms := NewMessageStore(database)

	conv, _ := cs.Create("user-1", "")
	ms.Save(conv.ID, "user", "msg1", "")
	ms.Save(conv.ID, "assistant", "msg2", "")
	ms.Save(conv.ID, "user", "msg3", "")

	msgs, err := ms.ListAfterMessage(conv.ID, "")
	if err != nil {
		t.Fatalf("ListAfterMessage: %v", err)
	}
	if len(msgs) != 3 {
		t.Errorf("len = %d, want 3", len(msgs))
	}
}

func TestListAfterMessage_WithID(t *testing.T) {
	database := setupTestDB(t)
	cs := NewConversationStore(database)
	ms := NewMessageStore(database)

	conv, _ := cs.Create("user-1", "")
	ms.Save(conv.ID, "user", "msg1", "")
	ms.Save(conv.ID, "assistant", "msg2", "")
	ms.Save(conv.ID, "user", "msg3", "")

	all, _ := ms.ListByConversation(conv.ID)
	if len(all) != 3 {
		t.Fatalf("setup: expected 3 messages, got %d", len(all))
	}
	msg1ID := all[0].ID

	after, err := ms.ListAfterMessage(conv.ID, msg1ID)
	if err != nil {
		t.Fatalf("ListAfterMessage: %v", err)
	}
	if len(after) != 2 {
		t.Errorf("len = %d, want 2", len(after))
	}
	if after[0].Content != "msg2" {
		t.Errorf("after[0].Content = %q, want %q", after[0].Content, "msg2")
	}
	if after[1].Content != "msg3" {
		t.Errorf("after[1].Content = %q, want %q", after[1].Content, "msg3")
	}
}
