package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"debtime/internal/format"
)

// frame draws a titled screen with a hint at the bottom. rows are plain
// lines already fitting the width; the cursor row is highlighted.
func (m *Model) frame(title string, rows []string, cursor int, hint string, extra ...string) string {
	w, h := m.width, m.height
	out := []string{m.st.bold().Render(truncate(title, w)), m.st.muted().Render(strings.Repeat("─", min(w, lipgloss.Width(title)+4)))}

	room := h - 3 - len(extra) // title, rule, hint
	if room < 1 {
		room = 1
	}
	offset := 0
	if len(rows) > room && cursor >= 0 {
		offset = min(max(cursor-room/2, 0), len(rows)-room)
	}
	for i := offset; i < len(rows) && i < offset+room; i++ {
		line := truncate(rows[i], w-2)
		if i == cursor {
			out = append(out, m.st.selected().Render("› "+line))
		} else {
			out = append(out, "  "+line)
		}
	}
	for _, e := range extra {
		out = append(out, m.st.muted().Render(truncate(e, w)))
	}
	out = append(out, m.st.muted().Render(truncate(hint, w)))
	return strings.Join(out, "\n")
}

// columns aligns a name and a right-hand value within width w.
func columns(name, value string, w int) string {
	if value == "" {
		return name
	}
	vw := lipgloss.Width(value)
	name = truncate(name, max(w-vw-2, 1))
	return name + strings.Repeat(" ", max(w-lipgloss.Width(name)-vw, 2)) + value
}

func (m *Model) partsCount(f format.Format) int {
	n := 0
	for _, s := range f.Steps {
		if s.Type != format.TeamPreparation {
			n++
		}
	}
	return n
}

func (m *Model) viewFormats() string {
	w := min(m.width-2, 60)
	var rows []string
	for _, id := range format.IDs() {
		f, _ := format.ByID(id, m.settings.FormatOptions())
		summary := m.tr.F("formatSummary", m.partsCount(f), longDuration(f.TotalSec()))
		rows = append(rows, columns(m.tr.FormatName(f), summary, w))
	}
	return m.frame(m.tr.T("titleFormats"), rows, m.cursor, m.tr.T("hintList"))
}

func (m *Model) viewSteps() string {
	e := m.eng
	f := e.Format()
	w := min(m.width-2, 70)
	var rows []string
	for i, s := range f.Steps {
		mark := "  "
		if i == e.Current() {
			mark = "▶ "
		}
		var value string
		if p := s.PrepPoolIndex; p != nil && *p < len(f.PrepPools) {
			value = m.tr.PoolName(f.PrepPools[*p]) + " " + clockText(secs(e.PoolRemaining(*p)))
		} else {
			value = clockText(s.DurationSec)
			if st := e.StepState(i); st.Started() {
				value = clockText(secs(e.StepElapsed(i))) + " / " + value
			}
		}
		name := fmt.Sprintf("%s%2d. %s", mark, i+1, m.tr.StepName(s))
		rows = append(rows, columns(name, value, w))
	}
	return m.frame(m.tr.T("titleSteps"), rows, m.cursor, m.tr.T("hintList"))
}

func (m *Model) viewPools() string {
	e := m.eng
	f := e.Format()
	if len(f.PrepPools) == 0 {
		return m.frame(m.tr.T("titlePools"), []string{m.tr.T("noPools")}, -1, m.tr.T("hintHelp"))
	}
	w := min(m.width-2, 50)
	var rows []string
	for i, p := range f.PrepPools {
		st := e.PoolState(i)
		status := m.tr.T("statusReady")
		switch {
		case st.Running:
			status = m.tr.T("statusRunning")
		case e.PoolExhausted(i):
			status = m.tr.T("statusExhausted")
		case st.Started():
			status = m.tr.T("statusPaused")
		}
		value := fmt.Sprintf("%s / %s   %s", clockText(secs(e.PoolRemaining(i))), clockText(p.DurationSec), status)
		rows = append(rows, columns(fmt.Sprintf("%d  %s", i+1, m.tr.PoolName(p)), value, w))
	}
	return m.frame(m.tr.T("titlePools"), rows, m.cursor, m.tr.T("hintPools"))
}

func (m *Model) viewSettings() string {
	w := min(m.width-2, 64)
	var rows []string
	for _, it := range m.settingItems() {
		rows = append(rows, columns(it.label, "‹ "+it.value+" ›", w))
	}
	return m.frame(m.tr.T("titleSettings"), rows, m.cursor, m.tr.T("hintSettings"), m.tr.T("settingResetNote"))
}

func (m *Model) viewHelp() string {
	tr := m.tr
	keys := [][2]string{
		{tr.T("helpSpaceKey"), tr.T("helpSpace")},
		{"← → / h l", tr.T("helpSteps")},
		{"n / Enter", tr.T("helpNext")},
		{"+ / -", tr.T("helpTime10")},
		{"] / [", tr.T("helpTime60")},
		{"r", tr.T("helpReset")},
		{"R", tr.T("helpRestart")},
		{"b", tr.T("helpBell")},
		{"1 / 2", tr.T("helpPools")},
		{"↑ ↓", tr.T("helpQuestions")},
		{"u", tr.T("helpCountUp")},
		{"f", tr.T("helpFormats")},
		{"L", tr.T("helpList")},
		{"p", tr.T("helpPoolScreen")},
		{"s", tr.T("helpSettings")},
		{"?", tr.T("helpHelp")},
		{"q", tr.T("helpQuit")},
	}
	var rows []string
	for _, k := range keys {
		rows = append(rows, padRight(k[0], 11)+" "+k[1])
	}
	return m.frame(tr.T("titleHelp"), rows, -1, tr.T("hintHelp"))
}
