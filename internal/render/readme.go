package render

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/alanisme/awesome-zed-extensions/internal/model"
	"github.com/alanisme/awesome-zed-extensions/internal/safefile"
)

func statusLabel(ext model.Extension) string {
	if ext.Archived {
		return "Archived"
	}
	if !ext.Maintained {
		return "Inactive"
	}
	return "Active"
}

const readmeTemplate = `<div align="center">

<h1>Awesome Zed Extensions</h1>

<p>
  <strong>Find the best extensions for <a href="https://zed.dev">Zed</a> — the high-performance code editor built in Rust.</strong><br>
  <strong>{{ .TotalCount }} extensions tracked, ranked by GitHub stars, updated daily.</strong>
</p>

<br>

<p>
  <a href="#-themes"><img src="https://img.shields.io/badge/Themes-{{ len .Themes }}-purple?style=for-the-badge&logo=palette&logoColor=white" alt="Themes"></a>&nbsp;
  <a href="#-languages"><img src="https://img.shields.io/badge/Languages-{{ len .Languages }}-green?style=for-the-badge&logo=code&logoColor=white" alt="Languages"></a>&nbsp;
  <a href="#-tools"><img src="https://img.shields.io/badge/Tools-{{ len .Tools }}-orange?style=for-the-badge&logo=wrench&logoColor=white" alt="Tools"></a>
</p>

<p>
  <sub>Fully automated · Data sourced from the <a href="https://github.com/zed-industries/extensions">official Zed extension registry</a> · Last update: <strong>{{ .UpdatedAt }}</strong></sub>
</p>

</div>

---

> **Why this list?** Zed's built-in extension marketplace doesn't show star counts, trending extensions, or category breakdowns. This project fills that gap — helping you discover the most popular and actively maintained extensions at a glance.

## Contents

- [Overview](#overview)
- [Top Extensions](#-top-extensions)
- [Trending This Week](#-trending-this-week)
- [Recently Added](#-recently-added)
- [Themes](#-themes)
- [Languages](#-languages)
- [Tools](#-tools)
- [Other](#-other)
- [About](#about)

## Overview

<p align="center">
  <img src="assets/top-extensions.svg" alt="Top Zed Extensions by Stars" width="600">
</p>

<p align="center">
  <img src="assets/categories.svg" alt="Zed Extension Categories" width="600">
</p>

---

## ⭐ Top Extensions

The most popular Zed extensions ranked by GitHub stars.

| # | Extension | Stars | Category | Status | Description |
|--:|-----------|------:|----------|--------|-------------|
{{- range $i, $e := .TopExtensions }}
| {{ inc $i }} | [{{ $e.Name }}]({{ $e.RepoURL }}) | {{ formatStars $e.Stars }} | {{ categoryBadge $e.Category }} | {{ statusBadge $e }} | {{ truncate $e.Description 120 }} |
{{- end }}

<div align="right"><sub><a href="#contents">↑ Back to top</a></sub></div>

---

## 📈 Trending This Week

Extensions gaining the most stars over the past 7 days.

{{ if .Trending -}}
| Extension | Stars | Growth | Description |
|-----------|------:|-------:|-------------|
{{- range $i, $t := .Trending }}
| [{{ $t.Name }}]({{ $t.RepoURL }}) | {{ formatStars $t.Stars }} | {{ growthBadge $t.Growth $i }} | {{ truncate $t.Description 120 }} |
{{- end }}
{{ else -}}
> No trending data yet. Trends will appear after the first week of tracking.
{{ end }}

<div align="right"><sub><a href="#contents">↑ Back to top</a></sub></div>

---

## 🆕 Recently Added

New extensions added to the Zed registry in the last 30 days.

{{ if .RecentlyAdded -}}
| Extension | Stars | Category | Description |
|-----------|------:|----------|-------------|
{{- range .RecentlyAdded }}
| [{{ .Name }}]({{ .RepoURL }}) | {{ formatStars .Stars }} | {{ categoryBadge .Category }} | {{ truncate .Description 120 }} |
{{- end }}
{{ else -}}
> No new extensions in the last 30 days.
{{ end }}

<div align="right"><sub><a href="#contents">↑ Back to top</a></sub></div>

---

## 🎨 Themes

Color themes and icon packs for Zed.

| # | Extension | Stars | Description |
|--:|-----------|------:|-------------|
{{- range $i, $e := .Themes }}
| {{ inc $i }} | [{{ $e.Name }}]({{ $e.RepoURL }}) | {{ formatStars $e.Stars }} | {{ truncate $e.Description 120 }} |
{{- end }}

<div align="right"><sub><a href="#contents">↑ Back to top</a></sub></div>

---

## 🌐 Languages

Programming language support — syntax highlighting, tree-sitter grammars, and language servers.

| # | Extension | Stars | Description |
|--:|-----------|------:|-------------|
{{- range $i, $e := .Languages }}
| {{ inc $i }} | [{{ $e.Name }}]({{ $e.RepoURL }}) | {{ formatStars $e.Stars }} | {{ truncate $e.Description 120 }} |
{{- end }}

<div align="right"><sub><a href="#contents">↑ Back to top</a></sub></div>

---

## 🔧 Tools

Developer tools — linters, formatters, LSP integrations, and productivity extensions.

| # | Extension | Stars | Description |
|--:|-----------|------:|-------------|
{{- range $i, $e := .Tools }}
| {{ inc $i }} | [{{ $e.Name }}]({{ $e.RepoURL }}) | {{ formatStars $e.Stars }} | {{ truncate $e.Description 120 }} |
{{- end }}

<div align="right"><sub><a href="#contents">↑ Back to top</a></sub></div>

---

## 📦 Other

Extensions that don't fit neatly into the categories above.

| # | Extension | Stars | Description |
|--:|-----------|------:|-------------|
{{- range $i, $e := .Others }}
| {{ inc $i }} | [{{ $e.Name }}]({{ $e.RepoURL }}) | {{ formatStars $e.Stars }} | {{ truncate $e.Description 120 }} |
{{- end }}

<div align="right"><sub><a href="#contents">↑ Back to top</a></sub></div>

---

## About

This directory is automatically generated from the official [Zed extensions registry](https://github.com/zed-industries/extensions). A Go program scans every registered extension, fetches its GitHub metadata, classifies it by type, and renders this page — fully automated, no manual curation.

**Data freshness:** Updated daily at ~06:00 UTC via GitHub Actions. Last update: **{{ .UpdatedAt }}**.

**Scope & exclusion rules:**
- Only *dedicated* Zed extensions are listed — repositories specifically built for Zed.
- Large general-purpose projects (5,000+ stars) that happen to bundle a Zed extension are excluded unless "zed" appears in their name, description, or topics.
- Built-in extensions shipped with Zed itself (from ` + "`zed-industries/zed`" + `) are excluded.
- **Status labels:** *Active* = pushed within the last year and not archived; *Inactive* = no push in over a year; *Archived* = marked archived on GitHub.

**How it works:**

1. Scans the official Zed extensions registry daily
2. Fetches star counts, descriptions, and metadata via the GitHub API
3. Categorizes each extension by analyzing its ` + "`extension.toml`" + ` structure
4. Tracks star history over time to surface trending extensions
5. Generates charts and rankings, then commits updates via GitHub Actions
6. Exports machine-readable data to ` + "`data/extensions.json`" + ` for reuse

**Contributing:** Found a miscategorized extension or have a suggestion? [Open an issue](https://github.com/alanisme/awesome-zed-extensions/issues). You can also submit a PR to ` + "`data/overrides.json`" + ` to fix classification edge cases.

## License

[MIT](LICENSE)
`

var funcMap = template.FuncMap{
	"inc": func(i int) int {
		return i + 1
	},
	"formatStars": func(stars int) string {
		if stars >= 1000 {
			return fmt.Sprintf("%.1fk", float64(stars)/1000)
		}
		return fmt.Sprintf("%d", stars)
	},
	"truncate": func(s string, max int) string {
		s = strings.ReplaceAll(s, "|", "-")
		s = strings.ReplaceAll(s, "\n", " ")
		if len(s) > max {
			return s[:max-3] + "..."
		}
		return s
	},
	"categoryBadge": func(cat string) string {
		switch cat {
		case "Theme":
			return "🎨 Theme"
		case "Language":
			return "🌐 Language"
		case "Tool":
			return "🔧 Tool"
		default:
			return "📦 Other"
		}
	},
	"statusBadge": func(ext model.Extension) string {
		return statusLabel(ext)
	},
	"growthBadge": func(growth, rank int) string {
		prefix := ""
		if rank < 3 {
			prefix = "🔥 "
		}
		return fmt.Sprintf("%s+%d", prefix, growth)
	},
	"shieldsDate": func(date string) string {
		// shields.io uses -- to represent a literal hyphen.
		return strings.ReplaceAll(date, "-", "--")
	},
}

// GenerateREADME renders the README.md file from the provided data.
// Uses atomic write to prevent partial output on crash.
func GenerateREADME(outputPath string, data model.RenderData) error {
	tmpl, err := template.New("readme").Funcs(funcMap).Parse(readmeTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	return safefile.WriteAtomic(outputPath, buf.Bytes(), 0644)
}
