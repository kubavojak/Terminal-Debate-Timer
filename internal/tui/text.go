package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// clockText formats seconds as m:ss.
func clockText(sec int) string {
	if sec < 0 {
		return "+" + clockText(-sec)
	}
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}

// longDuration formats seconds as m:ss or h:mm:ss.
func longDuration(sec int) string {
	if sec >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", sec/3600, sec/60%60, sec%60)
	}
	return clockText(sec)
}

// truncate cuts plain text to at most w cells, ending with … when cut.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > w-1 {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String() + "…"
}

// padRight pads plain or styled text with spaces to w cells.
func padRight(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// spread puts left and right text on one line of width w. The left part is
// shortened when both do not fit; styles are applied after measuring.
func spread(left, right string, w int, ls, rs lipgloss.Style) string {
	rw := lipgloss.Width(right)
	if rw+1 > w {
		return ls.Render(truncate(left, w))
	}
	left = truncate(left, w-rw-1)
	gap := w - lipgloss.Width(left) - rw
	if right == "" {
		return ls.Render(left)
	}
	return ls.Render(left) + strings.Repeat(" ", gap) + rs.Render(right)
}

// center centers plain or styled text in w cells.
func center(s string, w int) string {
	return lipgloss.PlaceHorizontal(w, lipgloss.Center, s)
}
