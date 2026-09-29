// Package format describes debate formats as plain data: ordered steps,
// their durations, POI windows, signals and shared preparation pools.
package format

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// StepType is the kind of a debate segment.
type StepType int

const (
	Preparation StepType = iota
	Speech
	Reply
	CrossExamination
	Questioning
	Consultation
	TeamPreparation
)

var stepTypeNames = [...]string{
	"preparation",
	"speech",
	"reply",
	"crossExamination",
	"questioning",
	"consultation",
	"teamPreparation",
}

func (t StepType) String() string {
	if t >= 0 && int(t) < len(stepTypeNames) {
		return stepTypeNames[t]
	}
	return fmt.Sprintf("StepType(%d)", int(t))
}

// IsSpeech reports whether the step is a speech or a reply speech.
func (t StepType) IsSpeech() bool { return t == Speech || t == Reply }

// IsQuestioning reports whether the step is a cross-examination or questioning.
func (t StepType) IsQuestioning() bool { return t == CrossExamination || t == Questioning }

// IsPreparation reports whether the step is some kind of preparation time.
func (t StepType) IsPreparation() bool {
	return t == Preparation || t == Consultation || t == TeamPreparation
}

// PoiWindow marks the part of a speech in which points of information are
// allowed. Time before FromSec and from ToSec on is protected.
type PoiWindow struct {
	FromSec int
	ToSec   int
}

// SignalType is ordered by prominence: single < double < continuous.
type SignalType int

const (
	SignalNone SignalType = iota
	SignalSingle
	SignalDouble
	SignalContinuous
)

func (t SignalType) String() string {
	switch t {
	case SignalSingle:
		return "single"
	case SignalDouble:
		return "double"
	case SignalContinuous:
		return "continuous"
	}
	return "none"
}

// Signal is a beep played when the timer reaches AtSec.
type Signal struct {
	AtSec int
	Type  SignalType
}

// PrepPool is a team's preparation time shared across the whole debate.
type PrepPool struct {
	NameKey     string
	Label       string
	DurationSec int
	Signals     []Signal
}

// Step is one timed segment of a debate.
type Step struct {
	Type        StepType
	DurationSec int
	// NameKey is an i18n key. If the translation contains %s, Label is
	// substituted into it, otherwise Label is appended.
	NameKey  string
	Label    string
	Asker    string
	Answerer string
	Poi      *PoiWindow
	// MaxQuestions enables the question counter (0..MaxQuestions).
	MaxQuestions *int
	// PrepPoolIndex is set on TeamPreparation steps: the step controls
	// the shared pool instead of its own time.
	PrepPoolIndex *int
	Signals       []Signal
}

// Format is a complete debate format.
type Format struct {
	ID        string
	NameKey   string
	Steps     []Step
	PrepPools []PrepPool
}

// TotalSec is the sum of all step durations (pools are not included).
func (f Format) TotalSec() int {
	total := 0
	for _, s := range f.Steps {
		total += s.DurationSec
	}
	return total
}

// Signature is a fingerprint of the format's timing structure. Saved timer
// state is restored only when the signatures match.
func (f Format) Signature() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s", f.ID, f.NameKey)
	for _, s := range f.Steps {
		fmt.Fprintf(&b, "|%s:%d", s.Type, s.DurationSec)
		if s.PrepPoolIndex != nil {
			fmt.Fprintf(&b, "@%d", *s.PrepPoolIndex)
		}
	}
	for _, p := range f.PrepPools {
		fmt.Fprintf(&b, "|pool:%s:%d", p.NameKey, p.DurationSec)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}

func intPtr(v int) *int { return &v }
