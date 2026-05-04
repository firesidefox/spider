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
