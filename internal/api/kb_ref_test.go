package api

import (
	"strings"
	"testing"

	"github.com/spiderai/spider/internal/models"
)

func TestParseKBRefs(t *testing.T) {
	tests := []struct {
		input string
		want  []kbRef
	}{
		{
			"@kb nginx重启",
			[]kbRef{{raw: "@kb", displayName: "知识库", groupName: "", docTitle: ""}},
		},
		{
			"@kb:运维手册 nginx重启",
			[]kbRef{{raw: "@kb:运维手册", displayName: "运维手册", groupName: "运维手册", docTitle: ""}},
		},
		{
			"@kb:运维手册/nginx配置说明 怎么限速",
			[]kbRef{{raw: "@kb:运维手册/nginx配置说明", displayName: "运维手册/nginx配置说明", groupName: "运维手册", docTitle: "nginx配置说明"}},
		},
		{
			"@kb:运维手册 和 @kb:网络配置/BGP路由表",
			[]kbRef{
				{raw: "@kb:运维手册", displayName: "运维手册", groupName: "运维手册", docTitle: ""},
				{raw: "@kb:网络配置/BGP路由表", displayName: "网络配置/BGP路由表", groupName: "网络配置", docTitle: "BGP路由表"},
			},
		},
		{"没有引用", nil},
	}
	for _, tt := range tests {
		got := parseKBRefs(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("input=%q: got %d refs, want %d", tt.input, len(got), len(tt.want))
			continue
		}
		for i, w := range tt.want {
			g := got[i]
			if g.raw != w.raw || g.displayName != w.displayName || g.groupName != w.groupName || g.docTitle != w.docTitle {
				t.Errorf("input=%q ref[%d]: got %+v, want %+v", tt.input, i, g, w)
			}
		}
	}
}

func TestFormatKBBlock(t *testing.T) {
	block := formatKBBlock("运维手册", []string{"nginx重启方法\nsystemctl restart nginx"})
	if !strings.Contains(block, "[知识库: 运维手册") {
		t.Errorf("block missing header, got: %s", block)
	}
	if !strings.Contains(block, "1条结果") {
		t.Errorf("block missing count, got: %s", block)
	}
}

func TestStripKBRefs(t *testing.T) {
	got := stripKBRefs("@kb:运维手册/nginx配置说明 怎么限速")
	if got != "怎么限速" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestExpandKBRefs_NoRefs(t *testing.T) {
	msg := expandKBRefs("普通消息", nil, nil, nil)
	if msg != "普通消息" {
		t.Errorf("unexpected: %q", msg)
	}
}

func TestExpandKBRefs_GlobalSearch(t *testing.T) {
	called := false
	search := func(query string, groupID *int) []*models.Document {
		called = true
		if groupID != nil {
			t.Error("expected nil groupID for @kb")
		}
		return []*models.Document{{Title: "结果文档", Content: "内容"}}
	}
	result := expandKBRefs("@kb nginx重启",
		func(name string) *int { return nil },
		func(groupID int, title string) *models.Document { return nil },
		search,
	)
	if !called {
		t.Error("search not called")
	}
	if !strings.Contains(result, "[知识库: 知识库") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExpandKBRefs_ExactDoc(t *testing.T) {
	docLookupCalled := false
	docLookup := func(groupID int, title string) *models.Document {
		docLookupCalled = true
		if groupID != 42 {
			t.Errorf("expected groupID 42, got %d", groupID)
		}
		if title != "nginx配置说明" {
			t.Errorf("expected title nginx配置说明, got %s", title)
		}
		return &models.Document{Title: "nginx配置说明", Content: "worker_processes auto;"}
	}
	gid := 42
	groupLookup := func(name string) *int {
		if name == "运维手册" {
			return &gid
		}
		return nil
	}
	result := expandKBRefs(
		"@kb:运维手册/nginx配置说明 怎么限速",
		groupLookup,
		docLookup,
		func(query string, groupID *int) []*models.Document { return nil },
	)
	if !docLookupCalled {
		t.Error("docLookup not called")
	}
	if !strings.Contains(result, "worker_processes auto;") {
		t.Errorf("expected doc content in result, got: %s", result)
	}
}

func TestExpandKBRefs_GroupSearch(t *testing.T) {
	searchCalled := false
	gid := 7
	search := func(query string, groupID *int) []*models.Document {
		searchCalled = true
		if groupID == nil || *groupID != 7 {
			t.Errorf("expected groupID 7, got %v", groupID)
		}
		return []*models.Document{{Title: "结果", Content: "内容"}}
	}
	groupLookup := func(name string) *int {
		if name == "运维手册" {
			return &gid
		}
		return nil
	}
	result := expandKBRefs(
		"@kb:运维手册 nginx重启",
		groupLookup,
		func(groupID int, title string) *models.Document { return nil },
		search,
	)
	if !searchCalled {
		t.Error("search not called")
	}
	if !strings.Contains(result, "[知识库: 运维手册") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExpandKBRefs_DuplicateRaw(t *testing.T) {
	callCount := 0
	search := func(query string, groupID *int) []*models.Document {
		callCount++
		return []*models.Document{{Title: "结果", Content: "内容"}}
	}
	result := expandKBRefs(
		"@kb nginx重启 和 @kb 怎么操作",
		func(name string) *int { return nil },
		func(groupID int, title string) *models.Document { return nil },
		search,
	)
	// 两个 @kb 都应被替换，不应有原始 @kb 残留
	if strings.Contains(result, "@kb") {
		t.Errorf("raw @kb should be replaced, got: %s", result)
	}
	_ = callCount
}
