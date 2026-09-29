package format

import "strconv"

// Format IDs used on the command line and in settings.
const (
	IDBritishParliamentary = "bp"
	IDKarlPopper           = "kp"
	IDWorldSchools         = "wsdc"
	IDResitelska           = "resitelska"
	IDSnemovni             = "snemovni"
	IDCustom               = "custom"
)

const (
	// Grace is the tolerance after the end of a step before the continuous beep.
	Grace = 15
	// KPGrace is the tolerance used by Karl Popper.
	KPGrace = 30
	// ContinuousBeepSec is how long the continuous beep lasts.
	ContinuousBeepSec = 5

	CustomMinSpeakers     = 1
	CustomMaxSpeakers     = 20
	CustomMinMinutes      = 1
	CustomMaxMinutes      = 15
	DefaultCustomSpeakers = 4
	DefaultCustomMinutes  = 5

	// Pool indexes of Karl Popper.
	PoolAffirmative = 0
	PoolNegative    = 1
)

// Options holds the user-configurable parts of the formats.
type Options struct {
	CustomSpeakers int
	CustomMinutes  int
	// Consultation inserts a 15 s consultation between parts of Řešitelská.
	Consultation bool
	// KPAskers maps a Karl Popper answerer (A1, N1, A2, N2) to the asker.
	KPAskers map[string]string
}

// DefaultOptions returns the default format options.
func DefaultOptions() Options {
	return Options{
		CustomSpeakers: DefaultCustomSpeakers,
		CustomMinutes:  DefaultCustomMinutes,
		KPAskers:       DefaultKPAskers(),
	}
}

// IDs lists all format IDs in display order.
func IDs() []string {
	return []string{IDBritishParliamentary, IDKarlPopper, IDWorldSchools, IDResitelska, IDSnemovni, IDCustom}
}

// ByID builds the format with the given ID.
func ByID(id string, opt Options) (Format, bool) {
	switch id {
	case IDBritishParliamentary:
		return BritishParliamentary(), true
	case IDKarlPopper:
		return KarlPopper(opt.KPAskers), true
	case IDWorldSchools:
		return WorldSchools(), true
	case IDResitelska:
		return Resitelska(opt.Consultation), true
	case IDSnemovni:
		return Snemovni(), true
	case IDCustom:
		return Custom(opt.CustomSpeakers, opt.CustomMinutes), true
	}
	return Format{}, false
}

// All builds every format in display order.
func All(opt Options) []Format {
	var out []Format
	for _, id := range IDs() {
		f, _ := ByID(id, opt)
		out = append(out, f)
	}
	return out
}

func preparationStep(duration int) Step {
	return Step{
		Type:        Preparation,
		DurationSec: duration,
		NameKey:     "preparation",
		Signals:     prepSignals(duration),
	}
}

// BritishParliamentary: 15 min preparation and eight 7 min speeches.
func BritishParliamentary() Format {
	speakers := []struct{ key, label string }{
		{"speakerPM", "OG 1"},
		{"speakerLO", "OO 1"},
		{"speakerDPM", "OG 2"},
		{"speakerDLO", "OO 2"},
		{"speakerMG", "CG 1"},
		{"speakerMO", "CO 1"},
		{"speakerGW", "CG 2"},
		{"speakerOW", "CO 2"},
	}
	steps := []Step{preparationStep(15 * 60)}
	for _, s := range speakers {
		steps = append(steps, Step{
			Type:        Speech,
			DurationSec: 7 * 60,
			NameKey:     s.key,
			Label:       s.label,
			Poi:         &PoiWindow{FromSec: 60, ToSec: 6 * 60},
			Signals:     standardSignals(7*60, []int{60, 6 * 60}, Grace),
		})
	}
	return Format{ID: IDBritishParliamentary, NameKey: "formatBP", Steps: steps}
}

// KPSpeakers is the Karl Popper speech order.
var KPSpeakers = []string{"A1", "N1", "A2", "N2", "A3", "N3"}

// KPAnswerers are the speakers who are cross-examined after their speech.
var KPAnswerers = []string{"A1", "N1", "A2", "N2"}

// DefaultKPAskers returns the default cross-examination pairs.
func DefaultKPAskers() map[string]string {
	return map[string]string{"A1": "N3", "N1": "A3", "A2": "N1", "N2": "A1"}
}

// KPTeam returns the pool index of the speaker's team.
func KPTeam(speaker string) int {
	if len(speaker) > 0 && speaker[0] == 'N' {
		return PoolNegative
	}
	return PoolAffirmative
}

// ValidKPAskers lists who may cross-examine the given answerer: the speakers
// of the other team.
func ValidKPAskers(answerer string) []string {
	var out []string
	for _, s := range KPSpeakers {
		if KPTeam(s) != KPTeam(answerer) {
			out = append(out, s)
		}
	}
	return out
}

// NormalizeKPAskers fills missing or invalid pairs with defaults.
func NormalizeKPAskers(askers map[string]string) map[string]string {
	def := DefaultKPAskers()
	out := make(map[string]string, len(KPAnswerers))
	for _, ans := range KPAnswerers {
		out[ans] = def[ans]
		if a, ok := askers[ans]; ok {
			for _, v := range ValidKPAskers(ans) {
				if v == a {
					out[ans] = a
				}
			}
		}
	}
	return out
}

func teamPrepStep(pool int) Step {
	key := "teamPrepAffirmative"
	if pool == PoolNegative {
		key = "teamPrepNegative"
	}
	return Step{Type: TeamPreparation, NameKey: key, PrepPoolIndex: intPtr(pool)}
}

// KarlPopper: 60 min preparation, six 6 min speeches, four 3 min
// cross-examinations and two 5 min team preparation pools.
func KarlPopper(askers map[string]string) Format {
	askers = NormalizeKPAskers(askers)
	pools := []PrepPool{
		{NameKey: "prepPoolAffirmative", Label: "A", DurationSec: 5 * 60, Signals: standardSignals(5*60, []int{4 * 60}, 0)},
		{NameKey: "prepPoolNegative", Label: "N", DurationSec: 5 * 60, Signals: standardSignals(5*60, []int{4 * 60}, 0)},
	}
	steps := []Step{preparationStep(60 * 60)}
	for _, sp := range KPSpeakers {
		steps = append(steps, teamPrepStep(KPTeam(sp)))
		steps = append(steps, Step{
			Type:        Speech,
			DurationSec: 6 * 60,
			NameKey:     "kpSpeaker",
			Label:       sp,
			Signals:     standardSignals(6*60, []int{5 * 60}, KPGrace),
		})
		if asker, ok := askers[sp]; ok {
			steps = append(steps, teamPrepStep(KPTeam(asker)))
			steps = append(steps, Step{
				Type:        CrossExamination,
				DurationSec: 3 * 60,
				NameKey:     "stepCrossExamination",
				Label:       asker + " → " + sp,
				Asker:       asker,
				Answerer:    sp,
				Signals:     standardSignals(3*60, []int{2 * 60}, KPGrace),
			})
		}
	}
	return Format{ID: IDKarlPopper, NameKey: "formatKP", Steps: steps, PrepPools: pools}
}

// WorldSchools: 60 min preparation, six 8 min speeches and two 4 min replies.
func WorldSchools() Format {
	steps := []Step{preparationStep(60 * 60)}
	for i := 1; i <= 3; i++ {
		for _, key := range []string{"speakerProp", "speakerOpp"} {
			steps = append(steps, Step{
				Type:        Speech,
				DurationSec: 8 * 60,
				NameKey:     key,
				Label:       strconv.Itoa(i),
				Poi:         &PoiWindow{FromSec: 60, ToSec: 7 * 60},
				Signals:     standardSignals(8*60, []int{60, 7 * 60}, Grace),
			})
		}
	}
	for _, key := range []string{"replyOpposition", "replyProposition"} {
		steps = append(steps, Step{
			Type:        Reply,
			DurationSec: 4 * 60,
			NameKey:     key,
			Signals:     standardSignals(4*60, []int{3 * 60}, Grace),
		})
	}
	return Format{ID: IDWorldSchools, NameKey: "formatWSDC", Steps: steps}
}

// lastMinuteSignals: single one minute before the end for steps longer than
// a minute, double at the end, continuous after the grace period.
func lastMinuteSignals(duration int) []Signal {
	var singles []int
	if duration > 60 {
		singles = []int{duration - 60}
	}
	return standardSignals(duration, singles, Grace)
}

// Resitelska: Řešitelská 2 × 2, 20 minutes without preparation. With
// consultation a 15 s consultation is inserted between the parts.
func Resitelska(consultation bool) Format {
	speech := func(label string, d int) Step {
		return Step{Type: Speech, DurationSec: d, NameKey: "stepSpeech", Label: label, Signals: lastMinuteSignals(d)}
	}
	question := func(asker, answerer string) Step {
		return Step{
			Type:        Questioning,
			DurationSec: 60,
			NameKey:     "stepQuestioning",
			Label:       asker + " × " + answerer,
			Asker:       asker,
			Answerer:    answerer,
			Signals:     lastMinuteSignals(60),
		}
	}
	parts := []Step{
		speech("H1", 3*60),
		question("O", "H"),
		speech("O1", 4*60),
		question("H", "O"),
		speech("H2", 4*60),
		question("O", "H"),
		speech("O2", 4*60),
		question("H", "O"),
		{Type: Reply, DurationSec: 60, NameKey: "stepConclusion", Label: "H1", Signals: lastMinuteSignals(60)},
	}
	var steps []Step
	for i, p := range parts {
		if consultation && i > 0 {
			steps = append(steps, Step{
				Type:        Consultation,
				DurationSec: 15,
				NameKey:     "stepConsultation",
				Signals:     standardSignals(15, nil, 0),
			})
		}
		steps = append(steps, p)
	}
	return Format{ID: IDResitelska, NameKey: "formatResitelska", Steps: steps}
}

// Snemovni: Sněmovní 2 × 2, four 4 min speeches and four 2 min
// questionings separated by 30 s preparations.
func Snemovni() Format {
	speech := func(key string) Step {
		return Step{Type: Speech, DurationSec: 4 * 60, NameKey: key, Signals: standardSignals(4*60, []int{3 * 60}, Grace)}
	}
	question := func(key, asker, answerer string) Step {
		return Step{
			Type:         Questioning,
			DurationSec:  2 * 60,
			NameKey:      key,
			Asker:        asker,
			Answerer:     answerer,
			MaxQuestions: intPtr(5),
			Signals:      standardSignals(2*60, []int{60}, Grace),
		}
	}
	parts := []Step{
		speech("snemovniDescriber1"),
		question("snemovniQuestionTeam2", "T2", "T1"),
		speech("snemovniDescriber2"),
		question("snemovniQuestionTeam1", "T1", "T2"),
		speech("snemovniIlluminator1"),
		question("snemovniQuestionTeam2", "T2", "T1"),
		speech("snemovniIlluminator2"),
		question("snemovniQuestionTeam1", "T1", "T2"),
	}
	var steps []Step
	for i, p := range parts {
		if i > 0 {
			steps = append(steps, Step{Type: Preparation, DurationSec: 30, NameKey: "preparation", Signals: prepSignals(30)})
		}
		steps = append(steps, p)
	}
	return Format{ID: IDSnemovni, NameKey: "formatSnemovni", Steps: steps}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Custom: 15 min preparation and N speeches of M minutes. Out-of-range
// values are clamped so corrupted settings never break the start.
func Custom(speakers, minutes int) Format {
	speakers = clamp(speakers, CustomMinSpeakers, CustomMaxSpeakers)
	minutes = clamp(minutes, CustomMinMinutes, CustomMaxMinutes)
	d := minutes * 60
	singles := []int{60}
	if d-60 > 60 {
		singles = append(singles, d-60)
	}
	steps := []Step{preparationStep(15 * 60)}
	for i := 1; i <= speakers; i++ {
		steps = append(steps, Step{
			Type:        Speech,
			DurationSec: d,
			NameKey:     "customSpeaker",
			Label:       strconv.Itoa(i),
			Signals:     standardSignals(d, singles, Grace),
		})
	}
	return Format{ID: IDCustom, NameKey: "formatCustom", Steps: steps}
}
