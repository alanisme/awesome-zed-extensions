package category

import (
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	CategoryTheme    = "Theme"
	CategoryLanguage = "Language"
	CategoryTool     = "Tool"
	CategoryOther    = "Other"
)

// Classify determines the category of a Zed extension by inspecting
// its extension.toml content and GitHub topics.
func Classify(extensionToml []byte, repoTopics []string) string {
	if len(extensionToml) == 0 {
		return classifyByTopics(repoTopics)
	}

	var parsed map[string]any
	if err := toml.Unmarshal(extensionToml, &parsed); err != nil {
		return classifyByTopics(repoTopics)
	}

	hasGrammars := hasSection(parsed, "grammars")
	hasLSP := hasSection(parsed, "language_servers")
	hasSlashCommands := hasSection(parsed, "slash_commands")
	hasContextServers := hasSection(parsed, "context_servers")
	isTheme := detectTheme(parsed, repoTopics)
	looksLikeLanguage := detectLanguage(parsed, repoTopics)

	switch {
	case hasGrammars:
		return CategoryLanguage
	case hasLSP && looksLikeLanguage:
		// LSP extensions whose name/description indicate language support
		// (e.g. "Java", "C# support") are Language, not Tool.
		return CategoryLanguage
	case hasLSP:
		return CategoryTool
	case hasSlashCommands || hasContextServers:
		return CategoryTool
	case isTheme:
		return CategoryTheme
	default:
		return classifyByTopics(repoTopics)
	}
}

func hasSection(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	sub, ok := v.(map[string]any)
	return ok && len(sub) > 0
}

func detectTheme(parsed map[string]any, topics []string) bool {
	for _, t := range topics {
		lower := strings.ToLower(t)
		if strings.Contains(lower, "theme") || strings.Contains(lower, "color-scheme") {
			return true
		}
	}

	if desc, ok := parsed["description"].(string); ok {
		lower := strings.ToLower(desc)
		if strings.Contains(lower, "theme") || strings.Contains(lower, "color") {
			return true
		}
	}

	if name, ok := parsed["name"].(string); ok {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "theme") {
			return true
		}
	}

	return false
}

// detectLanguage checks if the extension looks like a programming language
// support extension (as opposed to a general developer tool).
func detectLanguage(parsed map[string]any, topics []string) bool {
	// Common patterns in language extension names/descriptions.
	langKeywords := []string{
		"language", "support", "syntax", "highlighting",
		"extension for zed to support",
	}

	if desc, ok := parsed["description"].(string); ok {
		lower := strings.ToLower(desc)
		for _, kw := range langKeywords {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}

	if name, ok := parsed["name"].(string); ok {
		lower := strings.ToLower(name)
		// "X Language Server" pattern — common for language support extensions.
		if strings.Contains(lower, "language server") {
			return true
		}
	}

	for _, t := range topics {
		lower := strings.ToLower(t)
		if strings.Contains(lower, "language") || strings.Contains(lower, "syntax") || strings.Contains(lower, "grammar") {
			return true
		}
	}

	return false
}

func classifyByTopics(topics []string) string {
	for _, t := range topics {
		lower := strings.ToLower(t)
		switch {
		case strings.Contains(lower, "theme") || strings.Contains(lower, "color"):
			return CategoryTheme
		case strings.Contains(lower, "language") || strings.Contains(lower, "grammar") || strings.Contains(lower, "syntax"):
			return CategoryLanguage
		case strings.Contains(lower, "tool") || strings.Contains(lower, "lsp") || strings.Contains(lower, "linter") || strings.Contains(lower, "formatter"):
			return CategoryTool
		}
	}
	return CategoryOther
}
