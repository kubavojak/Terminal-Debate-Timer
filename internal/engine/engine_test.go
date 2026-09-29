package engine

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"debtime/internal/clock"
	"debtime/internal/format"
)

var t0 = time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)

func newBP() (*Engine, *clock.Fake) {
	c := clock.NewFake(t0)
	return New(c, format.BritishParliamentary()), c
}

func newKP() (*Engine, *clock.Fake) {
	c := clock.NewFake(t0)
	return New(c, format.KarlPopper(nil)), c
}

func signals(evs []Event) []format.Signal {
	var out []format.Signal
	for _, ev := range evs {
		if s, ok := ev.(SignalEvent); ok {
			out = append(out, s.Signal)
		}
	}
	return out
}

func has[T Event](evs []Event) bool {
	for _, ev := range evs {
		if _, ok := ev.(T); ok {
			return true
		}
	}
	return false
}

// tickFor advances the clock in 100 ms ticks and collects the signals.
func tickFor(e *Engine, c *clock.Fake, d time.Duration) []format.Signal {
	var out []format.Signal
	for step := time.Duration(0); step < d; step += 100 * time.Millisecond {
		c.Advance(100 * time.Millisecond)
		out = append(out, signals(e.Sync())...)
	}
	return out
}

func TestStartPauseKeepsFractions(t *testing.T) {
	e, c := newBP()
	for i := 0; i < 4; i++ {
		e.Start()
		c.Advance(750 * time.Millisecond)
		e.Pause()
		c.Advance(10 * time.Second) // paused time does not count
	}
	if got := e.StepElapsed(0); got != 3*time.Second {
		t.Fatalf("elapsed %v, want 3s", got)
	}
	st := e.StepState(0)
	if st.Running || !st.Paused {
		t.Fatalf("state %+v", st)
	}
}

func TestToggle(t *testing.T) {
	e, c := newBP()
	e.Toggle()
	if !e.StepState(0).Running {
		t.Fatal("toggle must start")
	}
	c.Advance(2 * time.Second)
	evs := e.Toggle()
	if e.StepState(0).Running || !has[CancelSignals](evs) {
		t.Fatal("toggle must pause and cancel signals")
	}
}

func TestStepChangeKeepsTimeAndPauses(t *testing.T) {
	e, c := newBP()
	e.GoTo(1)
	e.Start()
	c.Advance(90 * time.Second)
	evs := e.Next()
	if !has[CancelSignals](evs) {
		t.Error("step change must cancel signals")
	}
	if e.Current() != 2 {
		t.Fatalf("current %d", e.Current())
	}
	st := e.StepState(1)
	if st.Running || !st.Paused || e.StepElapsed(1) != 90*time.Second {
		t.Fatalf("previous step: %+v elapsed %v", st, e.StepElapsed(1))
	}
	c.Advance(time.Minute)
	e.Prev()
	if e.StepElapsed(1) != 90*time.Second {
		t.Fatalf("time must be kept, got %v", e.StepElapsed(1))
	}
	e.Start()
	c.Advance(10 * time.Second)
	if e.StepElapsed(1) != 100*time.Second {
		t.Fatalf("continue: got %v", e.StepElapsed(1))
	}
}

func TestGoToBounds(t *testing.T) {
	e, _ := newBP()
	if e.Prev() != nil || e.GoTo(99) != nil || e.GoTo(-1) != nil {
		t.Fatal("out of range moves must be ignored")
	}
	e.GoTo(8)
	if e.Next() != nil || e.Current() != 8 {
		t.Fatal("next on the last step must do nothing")
	}
}

func TestResetOnlyCurrentStep(t *testing.T) {
	e, c := newBP()
	e.Start()
	c.Advance(5 * time.Second)
	e.Next()
	e.Start()
	c.Advance(7 * time.Second)
	evs := e.Reset()
	if !has[CancelSignals](evs) {
		t.Error("reset must cancel signals")
	}
	if e.StepElapsed(1) != 0 || e.StepState(1).Running || e.StepState(1).Started() {
		t.Fatalf("current step not reset: %+v", e.StepState(1))
	}
	if e.StepElapsed(0) != 5*time.Second {
		t.Fatalf("other step changed: %v", e.StepElapsed(0))
	}
}

func TestFullRestart(t *testing.T) {
	e, c := newKP()
	e.GoTo(2)
	e.Start()
	c.Advance(time.Minute)
	e.TogglePrepPool(0)
	c.Advance(time.Minute)
	e.FullRestart()
	if e.Current() != 0 || e.AnyRunning() || e.StepElapsed(2) != 0 || e.PoolElapsed(0) != 0 {
		t.Fatal("full restart must reset everything")
	}
}

func TestOnlyOneStopwatchRuns(t *testing.T) {
	e, c := newKP()
	e.GoTo(2) // A1 speech
	e.Start()
	c.Advance(10 * time.Second)

	e.TogglePrepPool(format.PoolNegative)
	if e.StepState(2).Running {
		t.Fatal("starting a pool must pause the step")
	}
	c.Advance(5 * time.Second)
	e.TogglePrepPool(format.PoolAffirmative)
	if e.PoolState(format.PoolNegative).Running || !e.PoolState(format.PoolAffirmative).Running {
		t.Fatal("only one pool may run")
	}
	c.Advance(3 * time.Second)
	e.Start()
	if e.PoolState(format.PoolAffirmative).Running || !e.StepState(2).Running {
		t.Fatal("starting the step must pause the pool")
	}
	if e.StepElapsed(2) != 10*time.Second || e.PoolElapsed(1) != 5*time.Second || e.PoolElapsed(0) != 3*time.Second {
		t.Fatalf("times: step %v, N %v, A %v", e.StepElapsed(2), e.PoolElapsed(1), e.PoolElapsed(0))
	}
}

func TestPoolKeepsRunningOnStepChange(t *testing.T) {
	e, c := newKP()
	e.GoTo(1) // team preparation (A)
	e.Toggle()
	if !e.PoolState(format.PoolAffirmative).Running {
		t.Fatal("toggle on a team preparation step must start the pool")
	}
	if e.StepState(1).Started() {
		t.Fatal("team preparation step has no own time")
	}
	c.Advance(20 * time.Second)
	e.Next()
	c.Advance(10 * time.Second)
	if !e.PoolState(format.PoolAffirmative).Running || e.PoolElapsed(format.PoolAffirmative) != 30*time.Second {
		t.Fatalf("pool must keep running: %v", e.PoolElapsed(format.PoolAffirmative))
	}
}

func TestPoolStopsWhenExhausted(t *testing.T) {
	e, c := newKP()
	e.TogglePrepPool(format.PoolNegative)
	sigs := tickFor(e, c, 5*time.Minute+3*time.Second)
	p := e.PoolState(format.PoolNegative)
	if p.Running {
		t.Fatal("exhausted pool must stop")
	}
	if e.PoolElapsed(format.PoolNegative) != 5*time.Minute || e.PoolRemaining(format.PoolNegative) != 0 || p.Accumulated != 5*time.Minute {
		t.Fatalf("pool must stop exactly at its size, got %v", p.Accumulated)
	}
	want := []format.Signal{{AtSec: 240, Type: format.SignalSingle}, {AtSec: 300, Type: format.SignalDouble}}
	if len(sigs) != 2 || sigs[0] != want[0] || sigs[1] != want[1] {
		t.Fatalf("signals %v", sigs)
	}
	if e.TogglePrepPool(format.PoolNegative) != nil || e.PoolState(format.PoolNegative).Running {
		t.Fatal("exhausted pool cannot be started")
	}
	e.ResetPrepPool(format.PoolNegative)
	if e.PoolRemaining(format.PoolNegative) != 5*time.Minute {
		t.Fatal("reset must give the time back")
	}
}

func TestPoolExhaustedWithoutTicks(t *testing.T) {
	e, c := newKP()
	e.TogglePrepPool(0)
	c.Advance(7 * time.Minute) // no ticks in between
	evs := e.Sync()
	if e.PoolState(0).Running || e.PoolState(0).Accumulated != 5*time.Minute {
		t.Fatal("pool must stop at its size")
	}
	sigs := signals(evs)
	if len(sigs) != 1 || sigs[0].Type != format.SignalDouble {
		t.Fatalf("catch-up must play only the last signal, got %v", sigs)
	}
	if !has[StateChanged](evs) {
		t.Fatal("auto stop is a state change")
	}
}

func TestSignalsPlayOnTime(t *testing.T) {
	e, c := newBP()
	e.GoTo(1)
	e.Start()
	sigs := tickFor(e, c, 7*time.Minute+20*time.Second)
	want := []format.Signal{
		{AtSec: 60, Type: format.SignalSingle},
		{AtSec: 360, Type: format.SignalSingle},
		{AtSec: 420, Type: format.SignalDouble},
		{AtSec: 435, Type: format.SignalContinuous},
	}
	if len(sigs) != len(want) {
		t.Fatalf("signals %v", sigs)
	}
	for i := range want {
		if sigs[i] != want[i] {
			t.Fatalf("signal %d: %v, want %v", i, sigs[i], want[i])
		}
	}
}

func TestCatchUpPlaysOnlyLatestSignal(t *testing.T) {
	e, c := newBP()
	e.GoTo(1)
	e.Start()
	c.Advance(7*time.Minute + 5*time.Second) // missed 1:00, 6:00, 7:00
	sigs := signals(e.Sync())
	if len(sigs) != 1 || sigs[0] != (format.Signal{AtSec: 420, Type: format.SignalDouble}) {
		t.Fatalf("got %v", sigs)
	}
	if got := signals(e.Sync()); len(got) != 0 {
		t.Fatalf("nothing more in the same second: %v", got)
	}
}

func TestProcessSignalsTiePrefersProminent(t *testing.T) {
	s := &SpeakerState{}
	sigs := []format.Signal{{AtSec: 10, Type: format.SignalDouble}, {AtSec: 10, Type: format.SignalSingle}, {AtSec: 5, Type: format.SignalContinuous}}
	got, ok := processSignals(s, sigs, 12*time.Second)
	if !ok || got != (format.Signal{AtSec: 10, Type: format.SignalDouble}) {
		t.Fatalf("got %v", got)
	}
	if s.LastProcessedSec != 12 {
		t.Fatalf("last processed %d", s.LastProcessedSec)
	}
}

func TestSuspendIsCounted(t *testing.T) {
	e, c := newBP()
	e.Start()
	c.Advance(10 * time.Second)
	c.Suspend(3 * time.Minute)
	c.Advance(time.Second)
	if got := e.StepElapsed(0); got != 3*time.Minute+11*time.Second {
		t.Fatalf("got %v", got)
	}
}

func TestAdjustTime(t *testing.T) {
	e, c := newBP()
	e.GoTo(1)
	e.AdjustTime(30 * time.Second)
	if e.StepElapsed(1) != 30*time.Second || e.StepState(1).Running || !e.StepState(1).Paused {
		t.Fatalf("adjust stopped step: %v %+v", e.StepElapsed(1), e.StepState(1))
	}
	e.AdjustTime(-time.Minute)
	if e.StepElapsed(1) != 0 {
		t.Fatalf("never below zero, got %v", e.StepElapsed(1))
	}
	e.Start()
	c.Advance(5 * time.Second)
	e.AdjustTime(time.Minute)
	c.Advance(5 * time.Second)
	if e.StepElapsed(1) != 70*time.Second || !e.StepState(1).Running {
		t.Fatalf("adjust running step: %v", e.StepElapsed(1))
	}
	// The skipped 1:00 single is not played.
	if got := signals(e.Sync()); len(got) != 0 {
		t.Fatalf("skipped signals must not play: %v", got)
	}
}

func TestAdjustPoolTime(t *testing.T) {
	e, _ := newKP()
	e.GoTo(1) // team preparation A
	e.AdjustTime(10 * time.Minute)
	if e.PoolElapsed(0) != 5*time.Minute || e.PoolState(0).Running {
		t.Fatalf("pool adjust must cap at its size: %v", e.PoolElapsed(0))
	}
	e.AdjustTime(-time.Minute)
	if e.PoolRemaining(0) != time.Minute {
		t.Fatalf("remaining %v", e.PoolRemaining(0))
	}
}

func TestQuestionCounter(t *testing.T) {
	c := clock.NewFake(t0)
	e := New(c, format.Snemovni())
	if e.ChangeQuestions(1) != nil {
		t.Fatal("speech has no question counter")
	}
	e.GoTo(2)
	e.ChangeQuestions(-1)
	if e.StepState(2).QuestionCount != 0 {
		t.Fatal("counter must not go below 0")
	}
	for i := 0; i < 8; i++ {
		e.ChangeQuestions(1)
	}
	if e.StepState(2).QuestionCount != 5 {
		t.Fatalf("counter must stop at max, got %d", e.StepState(2).QuestionCount)
	}
}

func TestOnChangeCalled(t *testing.T) {
	e, _ := newBP()
	n := 0
	e.OnChange = func(*Engine) { n++ }
	e.Start()
	e.Pause()
	e.Next()
	e.Sync()
	if n != 3 {
		t.Fatalf("OnChange called %d times, want 3", n)
	}
}

func TestSetFormat(t *testing.T) {
	c := clock.NewFake(t0)
	e := New(c, format.KarlPopper(nil))
	e.GoTo(2)
	e.Start()
	c.Advance(time.Minute)
	kept, _ := e.SetFormat(format.KarlPopper(map[string]string{"A1": "N1"}))
	if !kept || e.StepElapsed(2) != time.Minute || e.Format().Steps[4].Label != "N1 → A1" {
		t.Fatal("same signature must keep state and update the format")
	}
	kept, _ = e.SetFormat(format.BritishParliamentary())
	if kept || e.Current() != 0 || e.AnyRunning() {
		t.Fatal("different signature must reset")
	}
}

func roundTrip(t *testing.T, s Snapshot) Snapshot {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var out Snapshot
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSnapshotRestoreRunning(t *testing.T) {
	e, c := newKP()
	e.GoTo(2)
	e.Start()
	c.Advance(90*time.Second + 500*time.Millisecond)
	e.GoTo(4)
	e.GoTo(2) // step paused at 90.5 s
	e.Start()
	c.Advance(10 * time.Second)
	e.TogglePrepPool(1) // pool N runs, step paused at 100.5 s
	c.Advance(20 * time.Second)
	snap := roundTrip(t, e.Snapshot())

	// The app is closed for two minutes.
	c2 := clock.NewFake(t0.Add(90*time.Second + 500*time.Millisecond + 30*time.Second + 2*time.Minute))
	e2 := New(c2, format.KarlPopper(nil))
	if err := e2.Restore(snap); err != nil {
		t.Fatal(err)
	}
	if e2.Current() != 2 || e2.StepElapsed(2) != 100*time.Second+500*time.Millisecond {
		t.Fatalf("step: current %d elapsed %v", e2.Current(), e2.StepElapsed(2))
	}
	if !e2.PoolState(1).Running || e2.PoolElapsed(1) != 20*time.Second+2*time.Minute {
		t.Fatalf("pool must keep running and count the gap: %v", e2.PoolElapsed(1))
	}
	if got := signals(e2.Sync()); len(got) != 0 {
		t.Fatalf("signals missed while closed must not play: %v", got)
	}
	c2.Advance(time.Minute)
	e2.Sync()
	if e2.PoolElapsed(1) != 3*time.Minute+20*time.Second {
		t.Fatalf("pool after restore: %v", e2.PoolElapsed(1))
	}
}

func TestRestoreExhaustsPool(t *testing.T) {
	e, _ := newKP()
	e.TogglePrepPool(0)
	snap := roundTrip(t, e.Snapshot())
	c2 := clock.NewFake(t0.Add(time.Hour))
	e2 := New(c2, format.KarlPopper(nil))
	if err := e2.Restore(snap); err != nil {
		t.Fatal(err)
	}
	if e2.PoolState(0).Running || e2.PoolRemaining(0) != 0 {
		t.Fatal("pool must be exhausted after a long gap")
	}
}

func TestRestoreRejectsOtherSignature(t *testing.T) {
	e, _ := newBP()
	snap := e.Snapshot()
	c := clock.NewFake(t0)
	if err := New(c, format.WorldSchools()).Restore(snap); !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("got %v", err)
	}
	if err := New(c, format.Custom(3, 5)).Restore(New(c, format.Custom(3, 6)).Snapshot()); !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("custom: got %v", err)
	}
}

func TestRestoreSanitizes(t *testing.T) {
	c := clock.NewFake(t0)
	e := New(c, format.Snemovni())
	snap := e.Snapshot()
	snap.Current = 99
	snap.Steps[2].QuestionCount = 42
	snap.Steps[0].QuestionCount = 3
	snap.Steps[1].ElapsedMs = -500
	w := t0
	snap.Steps[3].RunningSince = &w
	snap.Steps[4].RunningSince = &w
	if err := e.Restore(snap); err != nil {
		t.Fatal(err)
	}
	if e.Current() != len(e.Format().Steps)-1 {
		t.Errorf("current %d", e.Current())
	}
	if e.StepState(2).QuestionCount != 5 || e.StepState(0).QuestionCount != 0 {
		t.Error("question counts must be clamped")
	}
	if e.StepElapsed(1) != 0 {
		t.Error("negative time must clamp to 0")
	}
	if e.StepState(3).Running || e.StepState(4).Running {
		t.Error("non-current steps must not run")
	}

	snap.Steps = snap.Steps[:3]
	if err := e.Restore(snap); !errors.Is(err, ErrBadSnapshot) {
		t.Errorf("wrong length: %v", err)
	}
}
