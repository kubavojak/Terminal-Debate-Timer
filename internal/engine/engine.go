// Package engine holds the timer state of a debate. Time is computed from
// timestamps, never from the number of ticks. The engine plays nothing
// itself: actions and Sync return events for the UI.
package engine

import (
	"time"

	"debtime/internal/clock"
	"debtime/internal/format"
)

// SpeakerState is the stopwatch of one step or one preparation pool.
type SpeakerState struct {
	Accumulated  time.Duration
	RunStartedAt *clock.Reading
	Running      bool
	// Paused is set once the stopwatch was stopped with time on it; the UI
	// then offers "continue".
	Paused           bool
	LastProcessedSec int
	QuestionCount    int
}

// Elapsed returns the measured time at now.
func (s SpeakerState) Elapsed(now clock.Reading) time.Duration {
	d := s.Accumulated
	if s.Running && s.RunStartedAt != nil {
		d += clock.Between(*s.RunStartedAt, now)
	}
	if d < 0 {
		d = 0
	}
	return d
}

// Started reports whether the stopwatch has ever been used since reset.
func (s SpeakerState) Started() bool { return s.Running || s.Paused || s.Accumulated > 0 }

func (s *SpeakerState) start(now clock.Reading) {
	if s.Running {
		return
	}
	r := now
	s.RunStartedAt = &r
	s.Running = true
	s.Paused = false
}

func (s *SpeakerState) pause(now clock.Reading) {
	if !s.Running {
		return
	}
	s.Accumulated = s.Elapsed(now)
	s.RunStartedAt = nil
	s.Running = false
	s.Paused = true
}

// set replaces the elapsed time, keeping the running state.
func (s *SpeakerState) set(elapsed time.Duration, now clock.Reading) {
	s.Accumulated = elapsed
	if s.Running {
		r := now
		s.RunStartedAt = &r
	} else {
		s.Paused = elapsed > 0
	}
	s.LastProcessedSec = int(elapsed / time.Second)
}

// Source says whether an event belongs to a step or a pool.
type Source int

const (
	SourceStep Source = iota
	SourcePool
)

// Event is returned by actions and Sync.
type Event interface{ isEvent() }

// SignalEvent asks the UI to play a signal.
type SignalEvent struct {
	Source Source
	Index  int
	Signal format.Signal
}

// StateChanged means the state changed and was persisted.
type StateChanged struct{}

// CancelSignals asks the UI to stop a signal that is still playing
// (e.g. the continuous beep) after pause, reset or a step change.
type CancelSignals struct{}

func (SignalEvent) isEvent()   {}
func (StateChanged) isEvent()  {}
func (CancelSignals) isEvent() {}

// Engine is the state of the whole debate.
type Engine struct {
	clk     clock.Clock
	format  format.Format
	steps   []SpeakerState
	pools   []SpeakerState
	current int

	// OnChange is called after every state change (for persistence).
	OnChange func(*Engine)
}

// New returns an engine for the format with all timers at zero.
func New(clk clock.Clock, f format.Format) *Engine {
	e := &Engine{clk: clk}
	e.load(f)
	return e
}

func (e *Engine) load(f format.Format) {
	e.format = f
	e.steps = make([]SpeakerState, len(f.Steps))
	e.pools = make([]SpeakerState, len(f.PrepPools))
	e.current = 0
}

// SetFormat switches the format. When the timing structure is the same
// (same Signature) the state is kept, otherwise it starts from zero.
// It reports whether the state was kept.
func (e *Engine) SetFormat(f format.Format) (bool, []Event) {
	if f.Signature() == e.format.Signature() {
		e.format = f
		return true, e.changed()
	}
	e.load(f)
	return false, e.changed(CancelSignals{})
}

// Format returns the current format.
func (e *Engine) Format() format.Format { return e.format }

// Now reads the engine's clock.
func (e *Engine) Now() clock.Reading { return e.clk.Now() }

// Current returns the index of the current step.
func (e *Engine) Current() int { return e.current }

// CurrentStep returns the current step.
func (e *Engine) CurrentStep() format.Step { return e.format.Steps[e.current] }

// StepState returns the stopwatch of step i.
func (e *Engine) StepState(i int) SpeakerState { return e.steps[i] }

// PoolState returns the stopwatch of pool i.
func (e *Engine) PoolState(i int) SpeakerState { return e.pools[i] }

// StepElapsed returns the time measured on step i.
func (e *Engine) StepElapsed(i int) time.Duration { return e.steps[i].Elapsed(e.clk.Now()) }

// PoolElapsed returns the used time of pool i (never above its size).
func (e *Engine) PoolElapsed(i int) time.Duration {
	el := e.pools[i].Elapsed(e.clk.Now())
	if limit := e.poolDuration(i); el > limit {
		el = limit
	}
	return el
}

// PoolRemaining returns the remaining time of pool i.
func (e *Engine) PoolRemaining(i int) time.Duration { return e.poolDuration(i) - e.PoolElapsed(i) }

// PoolExhausted reports whether pool i is used up.
func (e *Engine) PoolExhausted(i int) bool { return e.PoolRemaining(i) <= 0 }

func (e *Engine) poolDuration(i int) time.Duration {
	return time.Duration(e.format.PrepPools[i].DurationSec) * time.Second
}

// AnyRunning reports whether a step or pool is running.
func (e *Engine) AnyRunning() bool {
	for _, s := range e.steps {
		if s.Running {
			return true
		}
	}
	for _, p := range e.pools {
		if p.Running {
			return true
		}
	}
	return false
}

// CurrentPool returns the pool controlled by the current step, if any.
func (e *Engine) CurrentPool() (int, bool) {
	st := e.CurrentStep()
	if st.Type == format.TeamPreparation && st.PrepPoolIndex != nil &&
		*st.PrepPoolIndex >= 0 && *st.PrepPoolIndex < len(e.pools) {
		return *st.PrepPoolIndex, true
	}
	return 0, false
}

func (e *Engine) changed(extra ...Event) []Event {
	if e.OnChange != nil {
		e.OnChange(e)
	}
	return append(extra, StateChanged{})
}

func (e *Engine) pausePools(now clock.Reading, except int) {
	for i := range e.pools {
		if i != except {
			e.pools[i].pause(now)
		}
	}
}

// Start starts the current step (or its pool on a team preparation step).
func (e *Engine) Start() []Event {
	if p, ok := e.CurrentPool(); ok {
		if e.pools[p].Running {
			return nil
		}
		return e.TogglePrepPool(p)
	}
	s := &e.steps[e.current]
	if s.Running {
		return nil
	}
	now := e.clk.Now()
	e.pausePools(now, -1)
	s.start(now)
	return e.changed()
}

// Pause pauses the current step (or its pool).
func (e *Engine) Pause() []Event {
	if p, ok := e.CurrentPool(); ok {
		if !e.pools[p].Running {
			return nil
		}
		return e.TogglePrepPool(p)
	}
	s := &e.steps[e.current]
	if !s.Running {
		return nil
	}
	s.pause(e.clk.Now())
	return e.changed(CancelSignals{})
}

// Toggle starts, pauses or continues the current step. On a team preparation
// step it controls the team's pool.
func (e *Engine) Toggle() []Event {
	if p, ok := e.CurrentPool(); ok {
		return e.TogglePrepPool(p)
	}
	if e.steps[e.current].Running {
		return e.Pause()
	}
	return e.Start()
}

// Reset clears the current step only. Team preparation steps have no time
// of their own; pools are reset with ResetPrepPool.
func (e *Engine) Reset() []Event {
	if _, ok := e.CurrentPool(); ok {
		return nil
	}
	e.steps[e.current] = SpeakerState{}
	return e.changed(CancelSignals{})
}

// GoTo moves to step i. A running step is paused, its time is kept.
// A running pool keeps running.
func (e *Engine) GoTo(i int) []Event {
	if i < 0 || i >= len(e.steps) || i == e.current {
		return nil
	}
	e.steps[e.current].pause(e.clk.Now())
	e.current = i
	return e.changed(CancelSignals{})
}

// Next moves to the next step.
func (e *Engine) Next() []Event { return e.GoTo(e.current + 1) }

// Prev moves to the previous step.
func (e *Engine) Prev() []Event { return e.GoTo(e.current - 1) }

// TogglePrepPool starts or pauses pool i. Starting it pauses the current
// step and other pools. An exhausted pool cannot be started.
func (e *Engine) TogglePrepPool(i int) []Event {
	if i < 0 || i >= len(e.pools) {
		return nil
	}
	now := e.clk.Now()
	p := &e.pools[i]
	if p.Running {
		p.pause(now)
		return e.changed(CancelSignals{})
	}
	if p.Elapsed(now) >= e.poolDuration(i) {
		return nil
	}
	e.steps[e.current].pause(now)
	e.pausePools(now, i)
	p.start(now)
	return e.changed()
}

// ResetPrepPool gives pool i its full time back.
func (e *Engine) ResetPrepPool(i int) []Event {
	if i < 0 || i >= len(e.pools) {
		return nil
	}
	e.pools[i] = SpeakerState{}
	return e.changed(CancelSignals{})
}

// ChangeQuestions changes the question counter within 0..MaxQuestions.
func (e *Engine) ChangeQuestions(delta int) []Event {
	st := e.CurrentStep()
	if st.MaxQuestions == nil {
		return nil
	}
	s := &e.steps[e.current]
	n := s.QuestionCount + delta
	if n < 0 {
		n = 0
	}
	if n > *st.MaxQuestions {
		n = *st.MaxQuestions
	}
	if n == s.QuestionCount {
		return nil
	}
	s.QuestionCount = n
	return e.changed()
}

// AdjustTime adds d (may be negative) to the elapsed time of the current
// step, or of its pool on a team preparation step. Elapsed time never goes
// below zero and a pool never beyond its size. Signals between the old and
// new time are skipped.
func (e *Engine) AdjustTime(d time.Duration) []Event {
	now := e.clk.Now()
	if p, ok := e.CurrentPool(); ok {
		s := &e.pools[p]
		el := clampDur(s.Elapsed(now)+d, 0, e.poolDuration(p))
		s.set(el, now)
		if el >= e.poolDuration(p) && s.Running {
			s.pause(now)
		}
		return e.changed(CancelSignals{})
	}
	s := &e.steps[e.current]
	s.set(clampDur(s.Elapsed(now)+d, 0, -1), now)
	return e.changed(CancelSignals{})
}

func clampDur(v, lo, hi time.Duration) time.Duration {
	if v < lo {
		return lo
	}
	if hi >= 0 && v > hi {
		return hi
	}
	return v
}

// FullRestart resets every step and pool and goes back to the first step.
func (e *Engine) FullRestart() []Event {
	e.load(e.format)
	return e.changed(CancelSignals{})
}

// Sync is called from the UI tick. State only changes when a whole second
// passes. Missed seconds are caught up but only one signal is returned:
// the latest one, the most prominent on a tie. An exhausted pool stops
// exactly at its size.
func (e *Engine) Sync() []Event {
	now := e.clk.Now()
	var evs []Event
	changed := false

	if s := &e.steps[e.current]; s.Running {
		if sg, ok := processSignals(s, e.format.Steps[e.current].Signals, s.Elapsed(now)); ok {
			evs = append(evs, SignalEvent{Source: SourceStep, Index: e.current, Signal: sg})
		}
	}
	for i := range e.pools {
		p := &e.pools[i]
		if !p.Running {
			continue
		}
		dur := e.poolDuration(i)
		el := p.Elapsed(now)
		if el >= dur {
			el = dur
			p.Accumulated = dur
			p.RunStartedAt = nil
			p.Running = false
			p.Paused = true
			changed = true
		}
		if sg, ok := processSignals(p, e.format.PrepPools[i].Signals, el); ok {
			evs = append(evs, SignalEvent{Source: SourcePool, Index: i, Signal: sg})
		}
	}
	if changed {
		evs = append(evs, e.changed()...)
	}
	return evs
}

func processSignals(s *SpeakerState, signals []format.Signal, elapsed time.Duration) (format.Signal, bool) {
	sec := int(elapsed / time.Second)
	if sec <= s.LastProcessedSec {
		return format.Signal{}, false
	}
	var best format.Signal
	found := false
	for _, sg := range signals {
		if sg.AtSec <= s.LastProcessedSec || sg.AtSec > sec {
			continue
		}
		if !found || sg.AtSec > best.AtSec || (sg.AtSec == best.AtSec && sg.Type > best.Type) {
			best = sg
			found = true
		}
	}
	s.LastProcessedSec = sec
	return best, found
}
