package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	mcppkg "github.com/spiderai/spider/internal/mcp"
)

var unsafeFilename = regexp.MustCompile(`[^\w\-. ]+`)

func safeFilename(title string) string {
	s := unsafeFilename.ReplaceAllString(title, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
	}
	if s == "" {
		s = "conversation"
	}
	return s
}

func buildMarkdown(title string, msgs []msgRow) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n> 导出时间：%s\n\n", title, time.Now().Format("2006-01-02 15:04"))
	for _, m := range msgs {
		if m.content == "" {
			continue
		}
		label := "User"
		if m.role == "assistant" {
			label = "Assistant"
		} else if m.role != "user" {
			continue
		}
		fmt.Fprintf(&sb, "---\n\n**%s**\n\n%s\n\n", label, m.content)
	}
	return sb.String()
}

type msgRow struct {
	role    string
	content string
}

func chatExportConversation(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	conv, err := verifyConvOwner(app, r, id)
	if err != nil {
		writeError(w, 404, "conversation not found")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "md"
	}
	if format != "md" && format != "json" {
		writeError(w, 400, "format must be md or json")
		return
	}

	msgs, err := app.MsgStore.ListByConversation(id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	base := safeFilename(conv.Title)

	switch format {
	case "md":
		rows := make([]msgRow, len(msgs))
		for i, m := range msgs {
			rows[i] = msgRow{role: m.Role, content: m.Content}
		}
		body := buildMarkdown(conv.Title, rows)
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, base))
		w.WriteHeader(200)
		fmt.Fprint(w, body)

	case "json":
		payload := map[string]any{
			"conversation": conv,
			"messages":     msgs,
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, base))
		writeJSON(w, 200, payload)
	}
}
