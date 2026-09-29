package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"debtime/internal/format"
)

// View renders the current screen, never wider or taller than the terminal.
func (m *Model) View() string {
	var out string
	switch m.screen {
	case screenFormats:
		out = m.viewFormats()
	case screenSteps:
		out = m.viewSteps()
	case screenPools:
		out = m.viewPools()
	case screenSettings:
		out = m.viewSettings()
	case screenHelp:
		out = m.viewHelp()
	default:
		out = m.viewMain()
	}
	return lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(out)
}

// display is what the big clock shows.
type display struct {
	text     string
	tone     tone
	overtime bool
	progress float64
	markers  []float64
	phase    string
	status   string
	running  bool
	started  bool
}

func secs(d time.Duration) int { return int(d / time.Second) }

func (m *Model) currentDisplay() display {
	e := m.eng
	step := e.CurrentStep()
	tr := m.tr

	if p, ok := e.CurrentPool(); ok {
		pool := e.Format().PrepPools[p]
		used := secs(e.PoolElapsed(p))
		st := e.PoolState(p)
		d := display{
			tone:     tonePrep,
			progress: float64(used) / float64(max(pool.DurationSec, 1)),
			phase:    tr.PoolName(pool),
			running:  st.Running,
			started:  st.Started(),
		}
		if m.settings.CountUp {
			d.text = clockText(used)
		} else {
			d.text = clockText(pool.DurationSec - used)
		}
		switch {
		case st.Running:
			d.status = tr.T("statusRunning")
		case e.PoolExhausted(p):
			d.status = tr.T("statusExhausted")
			d.tone = toneOvertime
		case st.Started():
			d.status = tr.T("statusPaused")
		default:
			d.status = tr.T("statusReady")
		}
		return d
	}

	st := e.StepState(e.Current())
	el := secs(e.StepElapsed(e.Current()))
	dur := step.DurationSec
	phase := format.SpeechPhaseFor(step, el)
	d := display{running: st.Running, started: st.Started(), overtime: phase == format.PhaseOvertime}

	if m.settings.CountUp {
		d.text = clockText(el)
	} else {
		d.text = clockText(dur - el)
	}
	if dur > 0 {
		d.progress = float64(el) / float64(dur)
		if step.Poi != nil {
			d.markers = []float64{float64(step.Poi.FromSec) / float64(dur), float64(step.Poi.ToSec) / float64(dur)}
		}
	}

	switch {
	case d.overtime:
		d.tone, d.phase = toneOvertime, tr.T("phaseOvertime")
	case step.Type.IsPreparation():
		d.tone, d.phase = tonePrep, tr.T("phasePreparation")
	case step.Type.IsQuestioning():
		d.tone, d.phase = toneQuestion, tr.T("phaseQuestioning")
	case step.Poi == nil:
		d.tone, d.phase = toneFree, tr.T("phaseSpeech")
	case phase == format.PhaseFree:
		d.tone, d.phase = toneFree, tr.T("phasePoi")
	default:
		d.tone, d.phase = toneProtected, tr.T("phaseProtected")
	}

	switch {
	case st.Running:
		d.status = tr.T("statusRunning")
	case st.Started():
		d.status = tr.T("statusPaused")
	default:
		d.status = tr.T("statusReady")
	}
	return d
}

// boxLine is a line of the main box: plain text for the flash (inverted)
// frame and the normally styled rendering.
type boxLine struct {
	plain    string
	rendered string
}

func plainLine(s string) boxLine { return boxLine{plain: s, rendered: s} }

func (m *Model) viewMain() string {
	w, h := m.width, m.height
	e := m.eng
	f := e.Format()
	step := e.CurrentStep()
	d := m.currentDisplay()
	tr := m.tr
	st := m.st

	if w < 20 || h < 10 {
		return strings.Join([]string{
			truncate(tr.StepName(step), w),
			st.fg(d.tone).Bold(true).Render(d.text),
			st.muted().Render(truncate(tr.T("tooSmall"), w)),
		}, "\n")
	}

	// Header
	name := tr.FormatName(f)
	if m.settings.CountUp {
		name += "  " + tr.T("countingUp")
	}
	header := spread(name, fmt.Sprintf("%d / %d", e.Current()+1, len(f.Steps)), w, st.bold(), st.muted())

	// Lines below the box
	var next string
	if e.Current()+1 < len(f.Steps) {
		next = tr.F("nextStep", tr.StepName(f.Steps[e.Current()+1]))
	} else {
		next = tr.T("lastStep")
	}
	nextLine := st.muted().Render(truncate(next, w))
	info := m.infoLine(w)
	footer := m.footer(w)

	inner := w - 4                         // border and one column of padding on each side
	fixed := 1 + 2 + 1 + 1 + 1 + 1 + 1 + 1 // header, border, title, 2 blanks, progress, next, footer
	if info != "" {
		fixed++
	}
	clockRows := h - fixed

	flash := m.flashOn()
	var lines []boxLine
	lines = append(lines, boxLine{
		plain:    spread(tr.StepName(step), d.status, inner, lipgloss.NewStyle(), lipgloss.NewStyle()),
		rendered: spread(tr.StepName(step), d.status, inner, st.bold(), st.muted()),
	})
	lines = append(lines, plainLine(""))
	lines = append(lines, m.clockLines(d, inner, clockRows)...)
	lines = append(lines, plainLine(""))
	lines = append(lines, m.progressLine(d, inner))

	var content []string
	for _, l := range lines {
		if flash {
			content = append(content, lipgloss.NewStyle().Reverse(true).Render(padRight(l.plain, inner)))
		} else {
			content = append(content, l.rendered)
		}
	}
	box := lipgloss.NewStyle().
		Border(st.border()).
		BorderForeground(st.color(d.tone)).
		Padding(0, 1).
		Width(w - 2).
		Render(strings.Join(content, "\n"))

	parts := []string{header, box, nextLine}
	if info != "" {
		parts = append(parts, info)
	}
	parts = append(parts, footer)
	return strings.Join(parts, "\n")
}

// clockLines draws the time as big digits when there is room: pixels 4×2
// cells, then 2×1 cells, then plain text (always below 40 columns).
func (m *Model) clockLines(d display, inner, rows int) []boxLine {
	style := m.st.fg(d.tone).Bold(true)
	if d.overtime && !m.blinkOn() {
		style = style.Faint(true)
	}
	if !d.running && d.started && !d.overtime {
		style = style.Faint(!m.st.highContrast)
	}
	if m.width >= 40 {
		for _, sc := range []struct{ px, py int }{{4, 2}, {2, 1}} {
			if rows < glyphRows*sc.py || bigWidth(d.text, sc.px) > inner {
				continue
			}
			var out []boxLine
			for _, l := range renderBig(d.text, sc.px, sc.py) {
				c := center(l, inner)
				out = append(out, boxLine{plain: c, rendered: style.Render(c)})
			}
			return out
		}
	}
	c := center(d.text, inner)
	return []boxLine{{plain: c, rendered: style.Render(c)}}
}

func (m *Model) progressLine(d display, inner int) boxLine {
	phase := d.phase
	barW := inner - lipgloss.Width(phase) - 4 // brackets and two spaces
	if barW < 6 {
		p := truncate(phase, inner)
		return boxLine{plain: p, rendered: m.st.fg(d.tone).Render(p)}
	}
	progress := d.progress
	if progress > 1 {
		progress = 1
	}
	if progress < 0 {
		progress = 0
	}
	filled := int(progress*float64(barW) + 0.5)
	bar := []rune(strings.Repeat("█", filled) + strings.Repeat("░", barW-filled))
	for _, mk := range d.markers {
		i := int(mk * float64(barW))
		if i > 0 && i < barW && i >= filled {
			bar[i] = '│'
		}
	}
	plain := "[" + string(bar) + "]  " + phase
	return boxLine{
		plain:    plain,
		rendered: m.st.fg(d.tone).Render("["+string(bar)+"]") + "  " + m.st.fg(d.tone).Bold(true).Render(phase),
	}
}

// infoLine shows the preparation pools and the question counter.
func (m *Model) infoLine(w int) string {
	e := m.eng
	f := e.Format()
	step := e.CurrentStep()
	var left []string
	var leftPlain []string
	if _, onPool := e.CurrentPool(); !onPool {
		for i, p := range f.PrepPools {
			s := m.tr.PoolName(p) + " " + clockText(secs(e.PoolRemaining(i)))
			if e.PoolState(i).Running {
				s += " ▶"
			}
			style := m.st.muted()
			if e.PoolState(i).Running {
				style = m.st.fg(tonePrep).Bold(true)
			}
			leftPlain = append(leftPlain, s)
			left = append(left, style.Render(s))
		}
	}
	var right string
	if step.MaxQuestions != nil {
		right = m.tr.F("questions", e.StepState(e.Current()).QuestionCount, *step.MaxQuestions)
	}
	if len(left) == 0 && right == "" {
		return ""
	}
	lp := strings.Join(leftPlain, "   ")
	if lipgloss.Width(lp)+lipgloss.Width(right)+1 > w {
		return spread(lp, right, w, m.st.muted(), m.st.bold())
	}
	gap := w - lipgloss.Width(lp) - lipgloss.Width(right)
	return strings.Join(left, "   ") + strings.Repeat(" ", gap) + m.st.bold().Render(right)
}

func (m *Model) footer(w int) string {
	tr := m.tr
	switch m.confirm {
	case confirmQuit:
		return m.st.fg(toneOvertime).Bold(true).Render(truncate(tr.T("confirmQuit"), w))
	case confirmRestart:
		return m.st.fg(toneOvertime).Bold(true).Render(truncate(tr.T("confirmRestart"), w))
	}
	d := m.currentDisplay()
	space := tr.T("keyStart")
	switch {
	case d.running:
		space = tr.T("keyPause")
	case d.started:
		space = tr.T("keyContinue")
	}
	items := []string{"␣ " + space}
	step := m.eng.CurrentStep()
	if step.MaxQuestions != nil {
		items = append(items, "↑/↓ "+tr.T("keyQuestions"), "⏎ "+tr.T("keyEndQuestioning"))
	}
	items = append(items, "←/→ "+tr.T("keyStep"), "+/- "+tr.T("keyTime"), "r "+tr.T("keyReset"))
	if len(m.eng.Format().PrepPools) > 0 {
		items = append(items, "1/2 "+tr.T("keyPools"))
	}
	items = append(items, "b "+tr.T("keyBell"), "? "+tr.T("keyHelp"))

	// Drop hints from the end until the line fits, but keep "? help".
	for len(items) > 1 && lipgloss.Width(strings.Join(items, "  ")) > w {
		if len(items) > 2 {
			items = append(items[:len(items)-2], items[len(items)-1])
		} else {
			items = items[:1]
		}
	}
	return m.st.muted().Render(truncate(strings.Join(items, "  "), w))
}
