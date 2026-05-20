package knowledge

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DocType represents the type of document being parsed.
type DocType string

const (
	DocTypeOpenAPI   DocType = "openapi"
	DocTypeMarkdown  DocType = "markdown"
	DocTypeUnknown   DocType = "unknown"
)

// ParsedEntry represents a single extracted entry from a document.
type ParsedEntry struct {
	Title   string // e.g., "GET /api/v1/query"
	Summary string // Brief description
	Content string // Full content (YAML/Markdown)
}

// Parser extracts structured entries from document content.
type Parser interface {
	Parse(ctx context.Context, content []byte, filename string) ([]ParsedEntry, error)
}

// DetectDocType determines the document type from content and filename.
func DetectDocType(content []byte, filename string) DocType {
	contentStr := string(content)

	// Check for OpenAPI/Swagger markers
	if hasYAMLMarker(contentStr, "openapi:") || hasYAMLMarker(contentStr, "swagger:") {
		return DocTypeOpenAPI
	}

	// Check JSON for OpenAPI
	if strings.Contains(filename, ".json") {
		if strings.Contains(contentStr, `"openapi"`) || strings.Contains(contentStr, `"swagger"`) {
			return DocTypeOpenAPI
		}
	}

	// Check for Markdown
	if strings.HasSuffix(filename, ".md") || strings.HasSuffix(filename, ".markdown") {
		return DocTypeMarkdown
	}

	return DocTypeUnknown
}

// hasYAMLMarker checks if content contains a YAML key at the start of a line.
func hasYAMLMarker(content, marker string) bool {
	idx := strings.Index(content, marker)
	if idx == -1 {
		return false
	}
	if idx == 0 {
		return true
	}
	return content[idx-1] == '\n'
}

// OpenAPIParser extracts entries from OpenAPI/Swagger documents.
type OpenAPIParser struct{}

// Parse extracts one entry per path+method combination from OpenAPI content.
func (p *OpenAPIParser) Parse(ctx context.Context, content []byte, filename string) ([]ParsedEntry, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Extract paths map
	pathsRaw, ok := doc["paths"]
	if !ok {
		return nil, fmt.Errorf("no 'paths' field found in OpenAPI document")
	}

	paths, ok := pathsRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("'paths' field is not a map")
	}

	var entries []ParsedEntry
	httpMethods := []string{"get", "post", "put", "delete", "patch", "options", "head"}

	// Iterate through each path
	for path, pathItemRaw := range paths {
		pathItem, ok := pathItemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Check each HTTP method
		for _, method := range httpMethods {
			operationRaw, exists := pathItem[method]
			if !exists {
				continue
			}

			operation, ok := operationRaw.(map[string]interface{})
			if !ok {
				continue
			}

			// Build entry
			entry := ParsedEntry{
				Title: fmt.Sprintf("%s %s", strings.ToUpper(method), path),
			}

			// Extract summary: operationId > summary > description (truncated)
			if opID, ok := operation["operationId"].(string); ok && opID != "" {
				entry.Summary = opID
			} else if summary, ok := operation["summary"].(string); ok && summary != "" {
				entry.Summary = summary
			} else if desc, ok := operation["description"].(string); ok && desc != "" {
				entry.Summary = truncate(desc, 100)
			}

			// Serialize operation to YAML for content
			operationYAML, err := yaml.Marshal(operation)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal operation: %w", err)
			}
			entry.Content = string(operationYAML)

			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// truncate truncates a string to maxLen characters, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
