package format

import (
	"reflect"
	"testing"
)

type stepSpec struct {
	typ   StepType
	dur   int
	name  string
	label string
}

func checkSteps(t *testing.T, f Format, want []stepSpec) {
	t.Helper()
	if len(f.Steps) != len(want) {
		t.Fatalf("%s: got %d steps, want %d", f.ID, len(f.Steps), len(want))
	}
	for i, w := range want {
		s := f.Steps[i]
		if s.Type != w.typ || s.DurationSec != w.dur || s.NameKey != w.name || s.Label != w.label {
			t.Errorf("%s step %d: got {%s %d %q %q}, want {%s %d %q %q}",
				f.ID, i, s.Type, s.DurationSec, s.NameKey, s.Label, w.typ, w.dur, w.name, w.label)
		}
	}
}

func sig(at int, typ SignalType) Signal { return Signal{AtSec: at, Type: typ} }

func TestStandardSignals(t *testing.T) {
	got := standardSignals(420, []int{360, 60, 0, 420, 500, 60}, 15)
	want := []Signal{sig(60, SignalSingle), sig(360, SignalSingle), sig(420, SignalDouble), sig(435, SignalContinuous)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := standardSignals(30, nil, 0); !reflect.DeepEqual(got, []Signal{sig(30, SignalDouble)}) {
		t.Fatalf("no grace: got %v", got)
	}
}

func TestPrepSignals(t *testing.T) {
	if got := prepSignals(900); !reflect.DeepEqual(got, []Signal{sig(840, SignalSingle), sig(900, SignalDouble)}) {
		t.Errorf("15 min: %v", got)
	}
	if got := prepSignals(120); !reflect.DeepEqual(got, []Signal{sig(120, SignalDouble)}) {
		t.Errorf("2 min: %v", got)
	}
	if got := prepSignals(30); !reflect.DeepEqual(got, []Signal{sig(30, SignalDouble)}) {
		t.Errorf("30 s: %v", got)
	}
}

func TestBritishParliamentary(t *testing.T) {
	f := BritishParliamentary()
	checkSteps(t, f, []stepSpec{
		{Preparation, 900, "preparation", ""},
		{Speech, 420, "speakerPM", "OG 1"},
		{Speech, 420, "speakerLO", "OO 1"},
		{Speech, 420, "speakerDPM", "OG 2"},
		{Speech, 420, "speakerDLO", "OO 2"},
		{Speech, 420, "speakerMG", "CG 1"},
		{Speech, 420, "speakerMO", "CO 1"},
		{Speech, 420, "speakerGW", "CG 2"},
		{Speech, 420, "speakerOW", "CO 2"},
	})
	if f.TotalSec() != 900+8*420 {
		t.Errorf("total %d", f.TotalSec())
	}
	wantSig := []Signal{sig(60, SignalSingle), sig(360, SignalSingle), sig(420, SignalDouble), sig(435, SignalContinuous)}
	for _, s := range f.Steps[1:] {
		if s.Poi == nil || *s.Poi != (PoiWindow{60, 360}) {
			t.Errorf("POI %v", s.Poi)
		}
		if !reflect.DeepEqual(s.Signals, wantSig) {
			t.Errorf("signals %v", s.Signals)
		}
	}
	if !reflect.DeepEqual(f.Steps[0].Signals, []Signal{sig(840, SignalSingle), sig(900, SignalDouble)}) {
		t.Errorf("prep signals %v", f.Steps[0].Signals)
	}
	if len(f.PrepPools) != 0 {
		t.Error("BP has no pools")
	}
}

func TestKarlPopper(t *testing.T) {
	f := KarlPopper(nil)
	a, n := PoolAffirmative, PoolNegative
	type kp struct {
		typ   StepType
		dur   int
		label string
		pool  int
	}
	want := []kp{
		{Preparation, 3600, "", -1},
		{TeamPreparation, 0, "", a}, {Speech, 360, "A1", -1}, {TeamPreparation, 0, "", n}, {CrossExamination, 180, "N3 → A1", -1},
		{TeamPreparation, 0, "", n}, {Speech, 360, "N1", -1}, {TeamPreparation, 0, "", a}, {CrossExamination, 180, "A3 → N1", -1},
		{TeamPreparation, 0, "", a}, {Speech, 360, "A2", -1}, {TeamPreparation, 0, "", n}, {CrossExamination, 180, "N1 → A2", -1},
		{TeamPreparation, 0, "", n}, {Speech, 360, "N2", -1}, {TeamPreparation, 0, "", a}, {CrossExamination, 180, "A1 → N2", -1},
		{TeamPreparation, 0, "", a}, {Speech, 360, "A3", -1},
		{TeamPreparation, 0, "", n}, {Speech, 360, "N3", -1},
	}
	if len(f.Steps) != len(want) {
		t.Fatalf("got %d steps, want %d", len(f.Steps), len(want))
	}
	for i, w := range want {
		s := f.Steps[i]
		if s.Type != w.typ || s.DurationSec != w.dur || s.Label != w.label {
			t.Errorf("step %d: got {%s %d %q}", i, s.Type, s.DurationSec, s.Label)
		}
		if w.pool >= 0 && (s.PrepPoolIndex == nil || *s.PrepPoolIndex != w.pool) {
			t.Errorf("step %d: pool %v, want %d", i, s.PrepPoolIndex, w.pool)
		}
		if w.pool < 0 && s.PrepPoolIndex != nil {
			t.Errorf("step %d: unexpected pool", i)
		}
		if s.Poi != nil {
			t.Errorf("step %d: KP has no POI", i)
		}
		switch s.Type {
		case Speech:
			if !reflect.DeepEqual(s.Signals, []Signal{sig(300, SignalSingle), sig(360, SignalDouble), sig(390, SignalContinuous)}) {
				t.Errorf("speech signals %v", s.Signals)
			}
		case CrossExamination:
			if !reflect.DeepEqual(s.Signals, []Signal{sig(120, SignalSingle), sig(180, SignalDouble), sig(210, SignalContinuous)}) {
				t.Errorf("cross signals %v", s.Signals)
			}
		}
	}
	if f.TotalSec() != 3600+6*360+4*180 {
		t.Errorf("total %d", f.TotalSec())
	}
	if len(f.PrepPools) != 2 {
		t.Fatalf("pools %d", len(f.PrepPools))
	}
	for _, p := range f.PrepPools {
		if p.DurationSec != 300 || !reflect.DeepEqual(p.Signals, []Signal{sig(240, SignalSingle), sig(300, SignalDouble)}) {
			t.Errorf("pool %+v", p)
		}
	}
}

func TestKarlPopperCustomAskers(t *testing.T) {
	f := KarlPopper(map[string]string{"A1": "N1", "N1": "A1", "A2": "A3" /* invalid: same team */})
	var labels []string
	for _, s := range f.Steps {
		if s.Type == CrossExamination {
			labels = append(labels, s.Label)
		}
	}
	want := []string{"N1 → A1", "A1 → N1", "N1 → A2", "A1 → N2"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("got %v, want %v", labels, want)
	}
	// Changing askers does not change the timing structure.
	if f.Signature() != KarlPopper(nil).Signature() {
		t.Error("askers must not change the signature")
	}
}

func TestWorldSchools(t *testing.T) {
	f := WorldSchools()
	checkSteps(t, f, []stepSpec{
		{Preparation, 3600, "preparation", ""},
		{Speech, 480, "speakerProp", "1"},
		{Speech, 480, "speakerOpp", "1"},
		{Speech, 480, "speakerProp", "2"},
		{Speech, 480, "speakerOpp", "2"},
		{Speech, 480, "speakerProp", "3"},
		{Speech, 480, "speakerOpp", "3"},
		{Reply, 240, "replyOpposition", ""},
		{Reply, 240, "replyProposition", ""},
	})
	if f.TotalSec() != 3600+6*480+2*240 {
		t.Errorf("total %d", f.TotalSec())
	}
	for _, s := range f.Steps[1:7] {
		if s.Poi == nil || *s.Poi != (PoiWindow{60, 420}) {
			t.Errorf("POI %v", s.Poi)
		}
		if !reflect.DeepEqual(s.Signals, []Signal{sig(60, SignalSingle), sig(420, SignalSingle), sig(480, SignalDouble), sig(495, SignalContinuous)}) {
			t.Errorf("signals %v", s.Signals)
		}
	}
	for _, s := range f.Steps[7:] {
		if s.Poi != nil {
			t.Error("reply has no POI")
		}
		if !reflect.DeepEqual(s.Signals, []Signal{sig(180, SignalSingle), sig(240, SignalDouble), sig(255, SignalContinuous)}) {
			t.Errorf("reply signals %v", s.Signals)
		}
	}
}

func TestResitelska(t *testing.T) {
	f := Resitelska(false)
	checkSteps(t, f, []stepSpec{
		{Speech, 180, "stepSpeech", "H1"},
		{Questioning, 60, "stepQuestioning", "O × H"},
		{Speech, 240, "stepSpeech", "O1"},
		{Questioning, 60, "stepQuestioning", "H × O"},
		{Speech, 240, "stepSpeech", "H2"},
		{Questioning, 60, "stepQuestioning", "O × H"},
		{Speech, 240, "stepSpeech", "O2"},
		{Questioning, 60, "stepQuestioning", "H × O"},
		{Reply, 60, "stepConclusion", "H1"},
	})
	if f.TotalSec() != 20*60 {
		t.Errorf("total %d, want 1200", f.TotalSec())
	}
	if !reflect.DeepEqual(f.Steps[0].Signals, []Signal{sig(120, SignalSingle), sig(180, SignalDouble), sig(195, SignalContinuous)}) {
		t.Errorf("H1 signals %v", f.Steps[0].Signals)
	}
	if !reflect.DeepEqual(f.Steps[1].Signals, []Signal{sig(60, SignalDouble), sig(75, SignalContinuous)}) {
		t.Errorf("questioning signals %v", f.Steps[1].Signals)
	}

	c := Resitelska(true)
	if len(c.Steps) != 17 {
		t.Fatalf("with consultation: %d steps, want 17", len(c.Steps))
	}
	for i, s := range c.Steps {
		isCons := i%2 == 1
		if isCons != (s.Type == Consultation) {
			t.Errorf("step %d: type %s", i, s.Type)
		}
		if isCons && (s.DurationSec != 15 || !reflect.DeepEqual(s.Signals, []Signal{sig(15, SignalDouble)})) {
			t.Errorf("consultation %+v", s)
		}
	}
	if c.TotalSec() != 20*60+8*15 {
		t.Errorf("total %d", c.TotalSec())
	}
	if c.Signature() == f.Signature() {
		t.Error("consultation must change the signature")
	}
}

func TestSnemovni(t *testing.T) {
	f := Snemovni()
	p := stepSpec{Preparation, 30, "preparation", ""}
	checkSteps(t, f, []stepSpec{
		{Speech, 240, "snemovniDescriber1", ""}, p,
		{Questioning, 120, "snemovniQuestionTeam2", ""}, p,
		{Speech, 240, "snemovniDescriber2", ""}, p,
		{Questioning, 120, "snemovniQuestionTeam1", ""}, p,
		{Speech, 240, "snemovniIlluminator1", ""}, p,
		{Questioning, 120, "snemovniQuestionTeam2", ""}, p,
		{Speech, 240, "snemovniIlluminator2", ""}, p,
		{Questioning, 120, "snemovniQuestionTeam1", ""},
	})
	if f.TotalSec() != 4*240+4*120+7*30 {
		t.Errorf("total %d", f.TotalSec())
	}
	for _, s := range f.Steps {
		switch s.Type {
		case Speech:
			if !reflect.DeepEqual(s.Signals, []Signal{sig(180, SignalSingle), sig(240, SignalDouble), sig(255, SignalContinuous)}) {
				t.Errorf("speech signals %v", s.Signals)
			}
		case Questioning:
			if s.MaxQuestions == nil || *s.MaxQuestions != 5 {
				t.Errorf("max questions %v", s.MaxQuestions)
			}
			if !reflect.DeepEqual(s.Signals, []Signal{sig(60, SignalSingle), sig(120, SignalDouble), sig(135, SignalContinuous)}) {
				t.Errorf("question signals %v", s.Signals)
			}
		case Preparation:
			if !reflect.DeepEqual(s.Signals, []Signal{sig(30, SignalDouble)}) {
				t.Errorf("prep signals %v", s.Signals)
			}
		}
	}
}

func TestCustom(t *testing.T) {
	f := Custom(3, 5)
	checkSteps(t, f, []stepSpec{
		{Preparation, 900, "preparation", ""},
		{Speech, 300, "customSpeaker", "1"},
		{Speech, 300, "customSpeaker", "2"},
		{Speech, 300, "customSpeaker", "3"},
	})
	if !reflect.DeepEqual(f.Steps[1].Signals, []Signal{sig(60, SignalSingle), sig(240, SignalSingle), sig(300, SignalDouble), sig(315, SignalContinuous)}) {
		t.Errorf("signals %v", f.Steps[1].Signals)
	}
	// 2 min: the second single (60) is not > 60, only one single.
	if got := Custom(1, 2).Steps[1].Signals; !reflect.DeepEqual(got, []Signal{sig(60, SignalSingle), sig(120, SignalDouble), sig(135, SignalContinuous)}) {
		t.Errorf("2 min signals %v", got)
	}
	// 1 min: no single at all.
	if got := Custom(1, 1).Steps[1].Signals; !reflect.DeepEqual(got, []Signal{sig(60, SignalDouble), sig(75, SignalContinuous)}) {
		t.Errorf("1 min signals %v", got)
	}
	for _, s := range f.Steps {
		if s.Poi != nil {
			t.Error("custom has no POI")
		}
	}
}

func TestCustomClamps(t *testing.T) {
	cases := []struct{ n, m, wantN, wantM int }{
		{0, 0, 1, 1},
		{-5, -5, 1, 1},
		{99, 99, 20, 15},
		{20, 15, 20, 15},
	}
	for _, c := range cases {
		f := Custom(c.n, c.m)
		if len(f.Steps)-1 != c.wantN || f.Steps[1].DurationSec != c.wantM*60 {
			t.Errorf("Custom(%d,%d): %d speeches of %d s", c.n, c.m, len(f.Steps)-1, f.Steps[1].DurationSec)
		}
	}
}

func TestByIDAndSignatures(t *testing.T) {
	seen := map[string]string{}
	for _, id := range IDs() {
		f, ok := ByID(id, DefaultOptions())
		if !ok || f.ID != id {
			t.Fatalf("ByID(%q) = %v, %v", id, f.ID, ok)
		}
		sig := f.Signature()
		if other, dup := seen[sig]; dup {
			t.Errorf("%s and %s share a signature", id, other)
		}
		seen[sig] = id
		if f.Signature() != sig {
			t.Error("signature must be deterministic")
		}
	}
	if _, ok := ByID("nope", DefaultOptions()); ok {
		t.Error("unknown ID must fail")
	}
	if Custom(3, 5).Signature() == Custom(3, 6).Signature() {
		t.Error("custom durations must change the signature")
	}
}

func TestSignalsSortedAndInsideSteps(t *testing.T) {
	for _, f := range All(Options{CustomSpeakers: 5, CustomMinutes: 7, Consultation: true}) {
		for i, s := range f.Steps {
			if s.Type == TeamPreparation {
				if len(s.Signals) != 0 || s.PrepPoolIndex == nil {
					t.Errorf("%s step %d: team prep must only reference a pool", f.ID, i)
				}
				continue
			}
			last := 0
			for _, sg := range s.Signals {
				if sg.AtSec < last {
					t.Errorf("%s step %d: signals not sorted", f.ID, i)
				}
				last = sg.AtSec
			}
			if n := len(s.Signals); n == 0 {
				t.Errorf("%s step %d: no signals", f.ID, i)
			}
		}
	}
}

func TestSpeechPhaseFor(t *testing.T) {
	bp := BritishParliamentary().Steps[1]
	cases := []struct {
		sec  int
		want SpeechPhase
	}{
		{0, PhaseProtectedStart},
		{59, PhaseProtectedStart},
		{60, PhaseFree},
		{359, PhaseFree},
		{360, PhaseProtectedEnd},
		{419, PhaseProtectedEnd},
		{420, PhaseOvertime},
		{500, PhaseOvertime},
	}
	for _, c := range cases {
		if got := SpeechPhaseFor(bp, c.sec); got != c.want {
			t.Errorf("BP at %d: %s, want %s", c.sec, got, c.want)
		}
	}
	kp := KarlPopper(nil).Steps[2]
	if SpeechPhaseFor(kp, 0) != PhaseFree || SpeechPhaseFor(kp, 359) != PhaseFree || SpeechPhaseFor(kp, 360) != PhaseOvertime {
		t.Error("speech without POI is free until overtime")
	}
	team := KarlPopper(nil).Steps[1]
	if SpeechPhaseFor(team, 100) != PhaseFree {
		t.Error("zero-length step never runs over")
	}
}
