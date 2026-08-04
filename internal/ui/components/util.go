package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func lipglossWidth(s string) int { return lipgloss.Width(s) }

func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

// PadLines makes a block exactly height rows tall so panels do not shrink when
// their content is short.
func PadLines(s string, height int) string {
	lines := strings.Split(s, "\n")
	if len(lines) >= height {
		return strings.Join(lines[:height], "\n")
	}
	return s + strings.Repeat("\n", height-len(lines))
}

// WrapWords breaks only on spaces. lipgloss also breaks on hyphens, which would
// split JVM flags like -XX:+UseG1GC across two lines.
func WrapWords(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	var out []string
	line := ""
	for _, word := range strings.Fields(s) {
		switch {
		case line == "":
			line = word
		case lipgloss.Width(line)+1+lipgloss.Width(word) <= width:
			line += " " + word
		default:
			out = append(out, line)
			line = word
		}
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// HumanCount abbreviates download counts: 2386810 becomes 2.4M.
func HumanCount(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
}

func HumanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	sec := int64(d.Seconds())
	switch {
	case sec < 60:
		return fmt.Sprintf("%ds", sec)
	case sec < 3600:
		return fmt.Sprintf("%dm %ds", sec/60, sec%60)
	case sec < 86400:
		return fmt.Sprintf("%dh %dm", sec/3600, (sec%3600)/60)
	default:
		return fmt.Sprintf("%dd %dh", sec/86400, (sec%86400)/3600)
	}
}

// ProgressBar renders a filled meter; ratio is clamped to [0,1].
func ProgressBar(ratio float64, width int, fill, empty lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio*float64(width) + 0.5)
	return fill.Render(strings.Repeat("█", filled)) +
		empty.Render(strings.Repeat("░", width-filled))
}

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

// Sparkline renders the tail of values scaled against max.
func Sparkline(values []float64, width int, max float64, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}
	if max <= 0 {
		max = 1
	}
	var b strings.Builder
	for i := 0; i < width-len(values); i++ {
		b.WriteRune(' ')
	}
	for _, v := range values {
		idx := int(v / max * float64(len(sparkRunes)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkRunes) {
			idx = len(sparkRunes) - 1
		}
		b.WriteRune(sparkRunes[idx])
	}
	return style.Render(b.String())
}
