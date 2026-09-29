package format

import "sort"

// standardSignals returns a single beep at every time from singleBeepsAt that
// lies strictly inside (0, duration), a double beep at duration and, when
// grace > 0, a continuous beep at duration+grace.
func standardSignals(duration int, singleBeepsAt []int, grace int) []Signal {
	singles := append([]int(nil), singleBeepsAt...)
	sort.Ints(singles)

	var out []Signal
	seen := map[int]bool{}
	for _, at := range singles {
		if at > 0 && at < duration && !seen[at] {
			seen[at] = true
			out = append(out, Signal{AtSec: at, Type: SignalSingle})
		}
	}
	out = append(out, Signal{AtSec: duration, Type: SignalDouble})
	if grace > 0 {
		out = append(out, Signal{AtSec: duration + grace, Type: SignalContinuous})
	}
	return out
}

// prepSignals: single one minute before the end (only for preparations
// longer than two minutes), double at the end, no continuous.
func prepSignals(duration int) []Signal {
	var singles []int
	if duration > 120 {
		singles = []int{duration - 60}
	}
	return standardSignals(duration, singles, 0)
}
