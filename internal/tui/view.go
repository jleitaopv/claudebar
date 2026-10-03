package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/jleitaopv/claudebar/internal/usage"
)

// defaultWidth is used until the terminal reports its size.
const defaultWidth = 40

// barColumn is where a window's bar starts: a leading space, an 8-cell label
// and a space. Detail lines are indented to the same column.
const barColumn = 1 + 8 + 1

// windowLineOverhead is the fixed width around a bar: the label column before
// it, then a space and a 5-cell percentage after it.
const windowLineOverhead = barColumn + 1 + 5

const minBarWidth = 10

// renderWindowLine renders one Usage Window as "label  bar  pct", exactly
// width cells wide (or wider when width is too small for minBarWidth).
func renderWindowLine(label string, w usage.UsageWindow, width int) string {
	barWidth := max(minBarWidth, width-windowLineOverhead)
	colour := lipgloss.NewStyle().Foreground(thresholdColour(w.Utilization))
	percent := colour.Render(fmt.Sprintf("%4.0f%%", w.Utilization))
	return fmt.Sprintf(" %-8s %s %s", label, renderBar(w.Utilization, barWidth, colour), percent)
}

func renderBar(utilization float64, width int, colour lipgloss.Style) string {
	fraction := math.Min(math.Max(utilization, 0), 100) / 100
	filled := int(math.Round(fraction * float64(width)))
	return colour.Render(strings.Repeat("█", filled)) + strings.Repeat("░", width-filled)
}

// detailIndent aligns a window's detail line under its bar.
var detailIndent = strings.Repeat(" ", barColumn)

// renderResetLine describes when w resets, relative to now and as a clock
// time in now's zone, e.g. "resets in 2h 14m (14:29)".
func renderResetLine(w usage.UsageWindow, now time.Time) string {
	if !w.Started() {
		return detailIndent + fmt.Sprintf("%.0f%% · starts with your next message", w.Utilization)
	}
	return detailIndent + fmt.Sprintf("resets in %s (%s)",
		formatDuration(w.ResetTime.Sub(now)), formatClockTime(w.ResetTime.In(now.Location()), now))
}

// formatDuration renders d using its two largest units: "2d 1h", "3h 0m",
// "5m 30s", or just "45s" under a minute.
func formatDuration(d time.Duration) string {
	d = max(d, 0).Truncate(time.Second)
	days := int(d / (24 * time.Hour))
	hours := int(d/time.Hour) % 24
	minutes := int(d/time.Minute) % 60
	seconds := int(d/time.Second) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

// formatClockTime renders t as "15:04" when it falls on now's calendar day,
// otherwise prefixed with the weekday, "Sun 15:04".
func formatClockTime(t, now time.Time) string {
	if t.YearDay() == now.YearDay() && t.Year() == now.Year() {
		return t.Format("15:04")
	}
	return t.Format("Mon 15:04")
}

// thresholdColour is claudebar's own warning scale. It deliberately ignores
// the endpoint's undocumented "severity" field.
func thresholdColour(utilization float64) color.Color {
	switch {
	case utilization >= 80:
		return lipgloss.Red
	case utilization >= 50:
		return lipgloss.Yellow
	default:
		return lipgloss.Green
	}
}

const keyHints = "r refresh  q quit"

// spreadLine puts left at the start and right at the end of a line width
// cells wide, keeping at least two spaces between them.
func spreadLine(left, right string, width int) string {
	gap := max(2, width-2-lipgloss.Width(left)-lipgloss.Width(right))
	return " " + left + strings.Repeat(" ", gap) + right + " "
}

func separator(width int) string {
	return " " + strings.Repeat("─", max(0, width-2))
}
