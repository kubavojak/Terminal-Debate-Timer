package engine

import (
	"errors"
	"time"

	"debtime/internal/clock"
)

// SnapshotVersion is the version of the saved state layout.
const SnapshotVersion = 1

// ErrSignatureMismatch means the saved state belongs to another format.
var ErrSignatureMismatch = errors.New("saved state belongs to a different format")

// ErrBadSnapshot means the saved state is inconsistent.
var ErrBadSnapshot = errors.New("saved state is inconsistent")

// TimerSnapshot is the saved form of a SpeakerState. A running stopwatch is
// saved as the elapsed time at SavedAt plus the wall-clock time it was
// saved at; after a restore it keeps running and the time in between counts.
type TimerSnapshot struct {
	ElapsedMs        int64      `json:"elapsedMs"`
	RunningSince     *time.Time `json:"runningSince,omitempty"`
	Paused           bool       `json:"paused,omitempty"`
	LastProcessedSec int        `json:"lastProcessedSec,omitempty"`
	QuestionCount    int        `json:"questionCount,omitempty"`
}

// Snapshot is the saved state of the whole debate.
type Snapshot struct {
	Version   int             `json:"version"`
	Signature string          `json:"signature"`
	FormatID  string          `json:"formatId"`
	Current   int             `json:"current"`
	Steps     []TimerSnapshot `json:"steps"`
	Pools     []TimerSnapshot `json:"pools,omitempty"`
}

// Snapshot captures the current state.
func (e *Engine) Snapshot() Snapshot {
	now := e.clk.Now()
	snap := func(s SpeakerState) TimerSnapshot {
		t := TimerSnapshot{
			ElapsedMs:        s.Elapsed(now).Milliseconds(),
			Paused:           s.Paused,
			LastProcessedSec: s.LastProcessedSec,
			QuestionCount:    s.QuestionCount,
		}
		if s.Running {
			w := now.Wall
			t.RunningSince = &w
		}
		return t
	}
	out := Snapshot{
		Version:   SnapshotVersion,
		Signature: e.format.Signature(),
		FormatID:  e.format.ID,
		Current:   e.current,
	}
	for _, s := range e.steps {
		out.Steps = append(out.Steps, snap(s))
	}
	for _, p := range e.pools {
		out.Pools = append(out.Pools, snap(p))
	}
	return out
}

// Restore loads a snapshot. It is refused when the format signature or the
// number of steps and pools differ. Signals that would have played while the
// app was closed are not played; at most one step or pool keeps running.
func (e *Engine) Restore(s Snapshot) error {
	if s.Signature != e.format.Signature() {
		return ErrSignatureMismatch
	}
	if len(s.Steps) != len(e.steps) || len(s.Pools) != len(e.pools) {
		return ErrBadSnapshot
	}
	now := e.clk.Now()
	load := func(t TimerSnapshot) SpeakerState {
		st := SpeakerState{
			Accumulated:   time.Duration(max(t.ElapsedMs, 0)) * time.Millisecond,
			Paused:        t.Paused,
			QuestionCount: max(t.QuestionCount, 0),
		}
		if t.RunningSince != nil {
			r := clock.FromWall(*t.RunningSince)
			st.RunStartedAt = &r
			st.Running = true
			st.Paused = false
		}
		return st
	}

	steps := make([]SpeakerState, len(s.Steps))
	pools := make([]SpeakerState, len(s.Pools))
	for i, t := range s.Steps {
		steps[i] = load(t)
	}
	for i, t := range s.Pools {
		pools[i] = load(t)
	}
	current := min(max(s.Current, 0), len(steps)-1)

	// Keep the invariant: only the current step or one pool runs.
	running := false
	for i := range steps {
		if steps[i].Running && (i != current || running) {
			steps[i].pause(now)
		}
		running = running || steps[i].Running
	}
	for i := range pools {
		if pools[i].Running && running {
			pools[i].pause(now)
		}
		running = running || pools[i].Running
	}

	for i := range steps {
		st := &steps[i]
		if m := e.format.Steps[i].MaxQuestions; m != nil {
			st.QuestionCount = min(st.QuestionCount, *m)
		} else {
			st.QuestionCount = 0
		}
		st.LastProcessedSec = int(st.Elapsed(now) / time.Second)
	}
	for i := range pools {
		p := &pools[i]
		dur := e.poolDuration(i)
		if p.Elapsed(now) >= dur {
			p.Accumulated = dur
			p.RunStartedAt = nil
			p.Running = false
			p.Paused = true
		}
		p.LastProcessedSec = int(p.Elapsed(now) / time.Second)
	}

	e.steps, e.pools, e.current = steps, pools, current
	return nil
}
