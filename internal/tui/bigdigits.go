package tui

import "strings"

// glyphs are 5 rows high; '#' is a lit pixel.
var glyphs = map[rune][5]string{
	'0': {"###", "# #", "# #", "# #", "###"},
	'1': {" # ", "## ", " # ", " # ", "###"},
	'2': {"###", "  #", "###", "#  ", "###"},
	'3': {"###", "  #", "###", "  #", "###"},
	'4': {"# #", "# #", "###", "  #", "  #"},
	'5': {"###", "#  ", "###", "  #", "###"},
	'6': {"###", "#  ", "###", "# #", "###"},
	'7': {"###", "  #", "  #", "  #", "  #"},
	'8': {"###", "# #", "###", "# #", "###"},
	'9': {"###", "# #", "###", "  #", "###"},
	':': {" ", "#", " ", "#", " "},
	'+': {"   ", " # ", "###", " # ", "   "},
	'-': {"   ", "   ", "###", "   ", "   "},
}

const glyphRows = 5

// bigWidth is the width in cells of s drawn with pixels px cells wide.
func bigWidth(s string, px int) int {
	w := 0
	n := 0
	for _, r := range s {
		g, ok := glyphs[r]
		if !ok {
			continue
		}
		w += len(g[0])
		n++
	}
	if n > 1 {
		w += n - 1 // one pixel gap between glyphs
	}
	return w * px
}

// renderBig draws s with pixels px cells wide and py rows high.
func renderBig(s string, px, py int) []string {
	on := strings.Repeat("█", px)
	off := strings.Repeat(" ", px)
	var lines []string
	for row := 0; row < glyphRows; row++ {
		var b strings.Builder
		first := true
		for _, r := range s {
			g, ok := glyphs[r]
			if !ok {
				continue
			}
			if !first {
				b.WriteString(off)
			}
			first = false
			for _, c := range g[row] {
				if c == '#' {
					b.WriteString(on)
				} else {
					b.WriteString(off)
				}
			}
		}
		// All rows keep the same width so the block stays aligned when centred.
		line := b.String()
		for i := 0; i < py; i++ {
			lines = append(lines, line)
		}
	}
	return lines
}
