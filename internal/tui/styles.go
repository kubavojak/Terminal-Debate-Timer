package tui

import "github.com/charmbracelet/lipgloss"

// tone is the meaning of a colour; the palette maps it to a real colour.
type tone int

const (
	toneNeutral tone = iota
	toneProtected
	toneFree
	toneOvertime
	tonePrep
	toneQuestion
)

type styles struct {
	noColor      bool
	highContrast bool
}

func (s styles) color(t tone) lipgloss.TerminalColor {
	if s.noColor {
		return lipgloss.NoColor{}
	}
	if s.highContrast {
		switch t {
		case toneProtected:
			return lipgloss.AdaptiveColor{Light: "3", Dark: "11"}
		case toneFree:
			return lipgloss.AdaptiveColor{Light: "2", Dark: "10"}
		case toneOvertime:
			return lipgloss.AdaptiveColor{Light: "1", Dark: "9"}
		case tonePrep:
			return lipgloss.AdaptiveColor{Light: "4", Dark: "14"}
		case toneQuestion:
			return lipgloss.AdaptiveColor{Light: "5", Dark: "13"}
		}
		return lipgloss.NoColor{}
	}
	switch t {
	case toneProtected:
		return lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#E3B341"}
	case toneFree:
		return lipgloss.AdaptiveColor{Light: "#1A7F37", Dark: "#3FB950"}
	case toneOvertime:
		return lipgloss.AdaptiveColor{Light: "#CF222E", Dark: "#F85149"}
	case tonePrep:
		return lipgloss.AdaptiveColor{Light: "#0969DA", Dark: "#58A6FF"}
	case toneQuestion:
		return lipgloss.AdaptiveColor{Light: "#8250DF", Dark: "#BC8CFF"}
	}
	return lipgloss.NoColor{}
}

// fg is text in the tone's colour (bold in high contrast mode).
func (s styles) fg(t tone) lipgloss.Style {
	st := lipgloss.NewStyle().Foreground(s.color(t))
	if s.highContrast {
		st = st.Bold(true)
	}
	return st
}

// muted is secondary text. High contrast mode does not dim anything.
func (s styles) muted() lipgloss.Style {
	if s.noColor || s.highContrast {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#57606A", Dark: "#8B949E"})
}

func (s styles) bold() lipgloss.Style { return lipgloss.NewStyle().Bold(true) }

func (s styles) border() lipgloss.Border {
	if s.highContrast {
		return lipgloss.ThickBorder()
	}
	return lipgloss.RoundedBorder()
}

// selected marks the cursor row in lists.
func (s styles) selected() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(s.color(tonePrep))
}
