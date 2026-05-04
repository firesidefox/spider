package api

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spiderai/spider/internal/models"
)

var kbRefRe = regexp.MustCompile(`@kb(?::([^\s/]+)(?:/([^\s]+))?)?`)

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
		displayName := "知识库"
		if groupName != "" && docTitle != "" {
			displayName = groupName + "/" + docTitle
		} else if groupName != "" {
			displayName = groupName
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
		var docs []*models.Document
		if ref.docTitle != "" {
			// exact lookup
			var groupID *int
			if ref.groupName != "" && groupLookup != nil {
				groupID = groupLookup(ref.groupName)
			}
			if groupID != nil {
				if doc := docLookup(*groupID, ref.docTitle); doc != nil {
					docs = []*models.Document{doc}
				}
			}
		} else {
			// vector search
			var groupID *int
			if ref.groupName != "" && groupLookup != nil {
				groupID = groupLookup(ref.groupName)
			}
			if search != nil {
				docs = search(query, groupID)
			}
		}
		var contents []string
		for _, d := range docs {
			contents = append(contents, d.Content)
		}
		block := formatKBBlock(ref.displayName, contents)
		message = strings.Replace(message, ref.raw, block, 1)
	}
	return message
}
