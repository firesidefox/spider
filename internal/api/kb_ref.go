package api

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spiderai/spider/internal/models"
)

var kbRefRe = regexp.MustCompile(`@kb:([^\s/]+)(?:/([^\s]+))?`)

type kbRef struct {
	raw         string
	displayName string
	groupName   string
	docTitle    string
}

func parseKBRefs(message string) []kbRef {
	matches := kbRefRe.FindAllStringSubmatch(message, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var refs []kbRef
	for _, m := range matches {
		raw := m[0]
		if seen[raw] {
			continue
		}
		seen[raw] = true
		groupName := m[1]
		docTitle := m[2]
		displayName := groupName
		if groupName != "" && docTitle != "" {
			displayName = groupName + "/" + docTitle
		}
		refs = append(refs, kbRef{
			raw:         raw,
			displayName: displayName,
			groupName:   groupName,
			docTitle:    docTitle,
		})
	}
	return refs
}

func formatKBBlock(displayName string, contents []string) string {
	header := fmt.Sprintf("[知识库: %s · %d条结果]", displayName, len(contents))
	if len(contents) == 0 {
		return header + "\n"
	}
	return header + "\n---\n" + strings.Join(contents, "\n\n") + "\n\n---\n"
}

func stripKBRefs(message string) string {
	return strings.TrimSpace(kbRefRe.ReplaceAllString(message, ""))
}

func expandKBRefs(
	message string,
	groupLookup func(name string) *int,
	docLookup func(groupID int, title string) *models.Document,
	search func(query string, groupID *int) []*models.Document,
) string {
	refs := parseKBRefs(message)
	if len(refs) == 0 {
		return message
	}
	query := stripKBRefs(message)
	for _, ref := range refs {
		var contents []string
		// resolve group if named
		var groupID *int
		groupMissing := false
		if ref.groupName != "" && groupLookup != nil {
			groupID = groupLookup(ref.groupName)
			if groupID == nil {
				groupMissing = true
			}
		}
		if groupMissing {
			contents = []string{"(分组不存在)"}
		} else if ref.docTitle != "" {
			// exact lookup
			if groupID != nil {
				if doc := docLookup(*groupID, ref.docTitle); doc != nil {
					contents = []string{doc.Content}
				}
			}
		} else {
			// vector search
			if search != nil {
				for _, d := range search(query, groupID) {
					contents = append(contents, d.Content)
				}
			}
		}
		block := formatKBBlock(ref.displayName, contents)
		message = strings.ReplaceAll(message, ref.raw, block)
	}
	return message
}
