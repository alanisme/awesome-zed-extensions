package registry

import (
	"strings"
	"testing"
)

func TestParseGitmodules(t *testing.T) {
	input := `[submodule "extensions/catppuccin"]
	path = extensions/catppuccin
	url = https://github.com/catppuccin/zed.git

[submodule "extensions/vim"]
	path = extensions/vim
	url = https://github.com/zed-extensions/vim
`
	modules, err := parseGitmodules(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(modules))
	}

	if modules["extensions/catppuccin"] != "https://github.com/catppuccin/zed" {
		t.Errorf("catppuccin URL wrong: %s", modules["extensions/catppuccin"])
	}

	if modules["extensions/vim"] != "https://github.com/zed-extensions/vim" {
		t.Errorf("vim URL wrong: %s", modules["extensions/vim"])
	}
}

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		url        string
		wantOwner  string
		wantRepo   string
	}{
		{"https://github.com/catppuccin/zed.git", "catppuccin", "zed"},
		{"https://github.com/catppuccin/zed", "catppuccin", "zed"},
		{"http://github.com/owner/repo", "owner", "repo"},
		{"https://codeberg.org/owner/repo", "", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		owner, repo := parseGitHubURL(tt.url)
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("parseGitHubURL(%q) = (%q, %q), want (%q, %q)",
				tt.url, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestParseGitmodules_StripsGitSuffix(t *testing.T) {
	input := `[submodule "extensions/test"]
	path = extensions/test
	url = https://github.com/owner/repo.git
`
	modules, err := parseGitmodules(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	url := modules["extensions/test"]
	if strings.HasSuffix(url, ".git") {
		t.Errorf("expected .git suffix stripped, got %s", url)
	}
}
