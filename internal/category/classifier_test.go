package category

import "testing"

func TestClassify_Grammars(t *testing.T) {
	toml := []byte(`
[grammars.python]
repository = "https://github.com/tree-sitter/tree-sitter-python"
`)
	got := Classify(toml, nil)
	if got != CategoryLanguage {
		t.Errorf("expected Language, got %s", got)
	}
}

func TestClassify_LSPWithLanguageDescription(t *testing.T) {
	toml := []byte(`
name = "Java"
description = "Extension for Zed to support Java"

[language_servers.jdtls]
language = "Java"
`)
	got := Classify(toml, nil)
	if got != CategoryLanguage {
		t.Errorf("expected Language, got %s", got)
	}
}

func TestClassify_LSPPureTool(t *testing.T) {
	toml := []byte(`
name = "Biome"
description = "Biome formatter and linter"

[language_servers.biome]
language = "JavaScript"
`)
	got := Classify(toml, nil)
	if got != CategoryTool {
		t.Errorf("expected Tool, got %s", got)
	}
}

func TestClassify_Theme(t *testing.T) {
	toml := []byte(`
name = "Catppuccin"
description = "Soothing pastel theme for Zed"
`)
	got := Classify(toml, nil)
	if got != CategoryTheme {
		t.Errorf("expected Theme, got %s", got)
	}
}

func TestClassify_ThemeByTopic(t *testing.T) {
	got := Classify(nil, []string{"zed-theme"})
	if got != CategoryTheme {
		t.Errorf("expected Theme, got %s", got)
	}
}

func TestClassify_SlashCommand(t *testing.T) {
	toml := []byte(`
name = "My Tool"
description = "A helpful tool"

[slash_commands.my-command]
`)
	got := Classify(toml, nil)
	if got != CategoryTool {
		t.Errorf("expected Tool, got %s", got)
	}
}

func TestClassify_Empty(t *testing.T) {
	got := Classify(nil, nil)
	if got != CategoryOther {
		t.Errorf("expected Other, got %s", got)
	}
}

func TestClassify_GrammarsAndLSP(t *testing.T) {
	toml := []byte(`
name = "Ruby"
description = "Ruby language support"

[grammars.ruby]
repository = "https://github.com/tree-sitter/tree-sitter-ruby"

[language_servers.solargraph]
language = "Ruby"
`)
	got := Classify(toml, nil)
	if got != CategoryLanguage {
		t.Errorf("expected Language, got %s", got)
	}
}

func TestClassify_LanguageServerInName(t *testing.T) {
	toml := []byte(`
name = "Eclipse JDT Language Server"
description = "Java development tools"

[language_servers.jdtls]
language = "Java"
`)
	got := Classify(toml, nil)
	if got != CategoryLanguage {
		t.Errorf("expected Language, got %s", got)
	}
}
