package store

import "testing"

func TestMessageStore_SaveAndList(t *testing.T) {
	database := setupTestDB(t)
	cs := NewConversationStore(database)
	ms := NewMessageStore(database)

	conv, _ := cs.Create("user-1", "test")

	err := ms.Save(conv.ID, "user", `{"type":"text","text":"hello"}`)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	err = ms.Save(conv.ID, "assistant", `{"type":"text","text":"hi"}`)
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
}

func TestMessageStore_DeleteByConversation(t *testing.T) {
	database := setupTestDB(t)
	cs := NewConversationStore(database)
	ms := NewMessageStore(database)

	conv, _ := cs.Create("user-1", "test")
	ms.Save(conv.ID, "user", `{"text":"hello"}`)

	err := ms.DeleteByConversation(conv.ID)
	if err != nil {
		t.Fatalf("DeleteByConversation: %v", err)
	}
	msgs, _ := ms.ListByConversation(conv.ID)
	if len(msgs) != 0 {
		t.Errorf("len = %d, want 0", len(msgs))
	}
}
