package render

import (
	"fmt"
	"math"
	"strings"

	"github.com/alanisme/awesome-zed-extensions/internal/model"
	"github.com/alanisme/awesome-zed-extensions/internal/safefile"
)

type categoryData struct {
	Name    string
	Count   int
	Color   string
	Percent float64
}

// GenerateCategoryChart creates an SVG pie chart of extension categories.
func GenerateCategoryChart(outputPath string, data model.RenderData) error {
	categories := []categoryData{
		{Name: "Themes", Count: len(data.Themes), Color: "#a855f7"},
		{Name: "Languages", Count: len(data.Languages), Color: "#22c55e"},
		{Name: "Tools", Count: len(data.Tools), Color: "#f97316"},
		{Name: "Other", Count: len(data.Others), Color: "#64748b"},
	}

	total := 0
	for i := range categories {
		total += categories[i].Count
	}
	for i := range categories {
		categories[i].Percent = float64(categories[i].Count) / float64(total) * 100
	}

	var svg strings.Builder
	svg.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 260" width="600" height="260">`)
	svg.WriteString(`<style>text{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif;fill:#e6edf3}`)
	svg.WriteString(`.label{font-size:13px}.count{font-size:12px;fill:#8b949e}.title{font-size:16px;font-weight:600}</style>`)
	svg.WriteString(`<rect width="600" height="260" rx="10" fill="#0d1117" stroke="#30363d" stroke-width="1"/>`)
	svg.WriteString(`<text x="300" y="30" text-anchor="middle" class="title">Extension Categories</text>`)

	// Draw pie chart.
	cx, cy, r := 150.0, 150.0, 90.0
	startAngle := -math.Pi / 2

	for _, cat := range categories {
		if cat.Count == 0 {
			continue
		}
		sweepAngle := 2 * math.Pi * float64(cat.Count) / float64(total)
		endAngle := startAngle + sweepAngle

		x1 := cx + r*math.Cos(startAngle)
		y1 := cy + r*math.Sin(startAngle)
		x2 := cx + r*math.Cos(endAngle)
		y2 := cy + r*math.Sin(endAngle)

		largeArc := 0
		if sweepAngle > math.Pi {
			largeArc = 1
		}

		svg.WriteString(fmt.Sprintf(
			`<path d="M%.1f,%.1f L%.1f,%.1f A%.0f,%.0f 0 %d,1 %.1f,%.1f Z" fill="%s" opacity="0.9"/>`,
			cx, cy, x1, y1, r, r, largeArc, x2, y2, cat.Color,
		))

		startAngle = endAngle
	}

	// Draw legend.
	legendX := 310.0
	legendY := 75.0
	for _, cat := range categories {
		svg.WriteString(fmt.Sprintf(
			`<rect x="%.0f" y="%.0f" width="14" height="14" rx="3" fill="%s" opacity="0.9"/>`,
			legendX, legendY-11, cat.Color,
		))
		svg.WriteString(fmt.Sprintf(
			`<text x="%.0f" y="%.0f" class="label">%s</text>`,
			legendX+22, legendY, cat.Name,
		))
		svg.WriteString(fmt.Sprintf(
			`<text x="%.0f" y="%.0f" class="count">%d (%.0f%%)</text>`,
			legendX+22, legendY+18, cat.Count, cat.Percent,
		))
		legendY += 46
	}

	svg.WriteString(`</svg>`)

	return safefile.WriteAtomic(outputPath, []byte(svg.String()), 0644)
}

// GenerateTopChart creates an SVG horizontal bar chart of top extensions by stars.
func GenerateTopChart(outputPath string, extensions []model.Extension) error {
	top := extensions
	if len(top) > 15 {
		top = top[:15]
	}
	if len(top) == 0 {
		return nil
	}

	maxStars := top[0].Stars
	barHeight := 24.0
	gap := 8.0
	leftPad := 200.0
	rightPad := 80.0
	chartWidth := 600.0
	barAreaWidth := chartWidth - leftPad - rightPad
	totalHeight := 50 + float64(len(top))*(barHeight+gap) + 20

	var svg strings.Builder
	svg.WriteString(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f">`,
		chartWidth, totalHeight, chartWidth, totalHeight,
	))
	svg.WriteString(`<style>text{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif;fill:#e6edf3}`)
	svg.WriteString(`.name{font-size:12px}.stars{font-size:11px;fill:#8b949e}.title{font-size:16px;font-weight:600}</style>`)
	svg.WriteString(fmt.Sprintf(
		`<rect width="%.0f" height="%.0f" rx="10" fill="#0d1117" stroke="#30363d" stroke-width="1"/>`,
		chartWidth, totalHeight,
	))
	svg.WriteString(fmt.Sprintf(
		`<text x="%.0f" y="30" text-anchor="middle" class="title">Top 15 Extensions by Stars</text>`,
		chartWidth/2,
	))

	for i, ext := range top {
		y := 50 + float64(i)*(barHeight+gap)
		barW := barAreaWidth * float64(ext.Stars) / float64(maxStars)
		if barW < 2 {
			barW = 2
		}

		// Truncate name.
		name := ext.Name
		if len(name) > 25 {
			name = name[:22] + "..."
		}

		color := categoryColor(ext.Category)

		svg.WriteString(fmt.Sprintf(
			`<text x="%.0f" y="%.1f" text-anchor="end" class="name">%s</text>`,
			leftPad-10, y+barHeight*0.7, name,
		))
		svg.WriteString(fmt.Sprintf(
			`<rect x="%.0f" y="%.1f" width="%.1f" height="%.0f" rx="4" fill="%s" opacity="0.85"/>`,
			leftPad, y, barW, barHeight, color,
		))
		svg.WriteString(fmt.Sprintf(
			`<text x="%.1f" y="%.1f" class="stars">%s</text>`,
			leftPad+barW+6, y+barHeight*0.7, formatStarsNum(ext.Stars),
		))
	}

	svg.WriteString(`</svg>`)

	return safefile.WriteAtomic(outputPath, []byte(svg.String()), 0644)
}

func categoryColor(cat string) string {
	switch cat {
	case "Theme":
		return "#a855f7"
	case "Language":
		return "#22c55e"
	case "Tool":
		return "#f97316"
	default:
		return "#64748b"
	}
}

func formatStarsNum(stars int) string {
	if stars >= 1000 {
		return fmt.Sprintf("%.1fk", float64(stars)/1000)
	}
	return fmt.Sprintf("%d", stars)
}
