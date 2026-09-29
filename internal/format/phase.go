package format

// SpeechPhase says which part of a step the timer is in.
type SpeechPhase int

const (
	PhaseProtectedStart SpeechPhase = iota
	PhaseFree
	PhaseProtectedEnd
	PhaseOvertime
)

func (p SpeechPhase) String() string {
	switch p {
	case PhaseProtectedStart:
		return "protectedStart"
	case PhaseFree:
		return "free"
	case PhaseProtectedEnd:
		return "protectedEnd"
	case PhaseOvertime:
		return "overtime"
	}
	return "unknown"
}

// SpeechPhaseFor is the only place that knows the protected-time rules.
// Steps without a POI window are free until they run over.
func SpeechPhaseFor(step Step, elapsedSec int) SpeechPhase {
	if step.DurationSec > 0 && elapsedSec >= step.DurationSec {
		return PhaseOvertime
	}
	if step.Poi == nil {
		return PhaseFree
	}
	if elapsedSec < step.Poi.FromSec {
		return PhaseProtectedStart
	}
	if elapsedSec >= step.Poi.ToSec {
		return PhaseProtectedEnd
	}
	return PhaseFree
}
