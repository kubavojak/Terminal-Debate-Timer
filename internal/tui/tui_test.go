package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"debtime/internal/clock"
	"debtime/internal/engine"
	"debtime/internal/format"
	"debtime/internal/sound"
	"debtime/internal/store"
)

var update = flag.Bool("update", false, "rewrite golden files")

var t0 = time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)

type fakeSound struct {
	mu     sync.Mutex
	played []format.SignalType
	stops  int
}

func (f *fakeSound) Play(t format.SignalType) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.played = append(f.played, t)
}
func (f *fakeSound) Stop()           { f.mu.Lock(); f.stops++; f.mu.Unlock() }
func (f *fakeSound) Close()          { f.Stop() }
func (f *fakeSound) Backend() string { return "fake" }

type harness struct {
	m     *Model
	clk   *clock.Fake
	snd   *fakeSound
	saved []store.Settings
}

func newHarness(t *testing.T, f format.Format, w, h int) *harness {
	t.Helper()
	hs := &harness{clk: clock.NewFake(t0), snd: &fakeSound{}}
	settings := store.DefaultSettings()
	settings.Lang = "cs"
	settings.Format = f.ID
	hs.m = New(Config{
		Engine:       engine.New(hs.clk, f),
		Settings:     settings,
		Sound:        hs.snd,
		SaveSettings: func(s store.Settings) { hs.saved = append(hs.saved, s) },
		NoColor:      true,
		Now:          func() time.Time { return hs.clk.Now().Wall },
	})
	hs.m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return hs
}

func (hs *harness) press(keys ...string) tea.Cmd {
	var cmd tea.Cmd
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case " ":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "left":
			msg = tea.KeyMsg{Type: tea.KeyLeft}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "ctrl+c":
			msg = tea.KeyMsg{Type: tea.KeyCtrlC}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		_, cmd = hs.m.Update(msg)
	}
	return cmd
}

// advance moves the clock in ticks like the real loop.
func (hs *harness) advance(d time.Duration) {
	for s := time.Duration(0); s < d; s += TickInterval {
		hs.clk.Advance(TickInterval)
		hs.m.Update(tickMsg(hs.clk.Now().Wall))
	}
}

func checkFits(t *testing.T, view string, w, h int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) > h {
		t.Errorf("view has %d lines, terminal has %d", len(lines), h)
	}
	for i, l := range lines {
		if lw := lipgloss.Width(l); lw > w {
			t.Errorf("line %d is %d wide (max %d): %q", i, lw, w, l)
		}
	}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no golden file %s (run go test ./internal/tui -update)", path)
	}
	if string(want) != got {
		t.Errorf("%s differs from golden file:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestViewFitsTerminal(t *testing.T) {
	sizes := []struct{ w, h int }{{80, 24}, {40, 12}, {120, 40}, {60, 18}, {30, 10}, {20, 8}}
	for _, f := range format.All(format.Options{CustomSpeakers: 3, CustomMinutes: 4, Consultation: true}) {
		for _, sz := range sizes {
			hs := newHarness(t, f, sz.w, sz.h)
			for _, keys := range [][]string{nil, {"l"}, {"l", "l", " "}, {"?"}, {"L"}, {"s"}, {"f"}, {"p"}} {
				hs.press("esc")
				hs.press(keys...)
				hs.advance(time.Second)
				checkFits(t, hs.m.View(), sz.w, sz.h)
			}
		}
	}
}

func TestMainViewGolden(t *testing.T) {
	for _, sz := range []struct{ w, h int }{{80, 24}, {40, 12}} {
		hs := newHarness(t, format.BritishParliamentary(), sz.w, sz.h)
		hs.press("l", "l", " ")
		hs.advance(65 * time.Second)
		view := hs.m.View()
		checkFits(t, view, sz.w, sz.h)
		if !strings.Contains(view, "3 / 9") {
			t.Errorf("%dx%d: step counter missing:\n%s", sz.w, sz.h, view)
		}
		golden(t, fmt.Sprintf("bp_main_%dx%d", sz.w, sz.h), view)
	}
}

func TestMainViewContent(t *testing.T) {
	hs := newHarness(t, format.BritishParliamentary(), 80, 24)
	hs.press("l", "l")
	view := hs.m.View()
	for _, want := range []string{"British Parliamentary", "3 / 9", "Vůdce opozice · OO 1", "Další: Místopremiér · OG 2", "připraveno", "Chráněný čas", "█"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in:\n%s", want, view)
		}
	}
	hs.press(" ")
	hs.advance(61 * time.Second)
	view = hs.m.View()
	if !strings.Contains(view, "POI povoleno") || !strings.Contains(view, "běží") {
		t.Errorf("free phase not shown:\n%s", view)
	}
	hs.press("u")
	if !hs.m.Settings().CountUp || len(hs.saved) == 0 {
		t.Fatal("u must toggle counting up and save")
	}
}

func TestNarrowTerminalUsesText(t *testing.T) {
	hs := newHarness(t, format.BritishParliamentary(), 38, 20)
	view := hs.m.View()
	if strings.Contains(view, "█") {
		t.Fatalf("narrow terminal must not use big digits:\n%s", view)
	}
	if !strings.Contains(view, "15:00") {
		t.Fatalf("time as text missing:\n%s", view)
	}
}

func TestSpaceStartsAndPauses(t *testing.T) {
	hs := newHarness(t, format.BritishParliamentary(), 80, 24)
	e := hs.m.eng
	hs.press(" ")
	if !e.StepState(0).Running {
		t.Fatal("space must start")
	}
	hs.advance(3 * time.Second)
	hs.press(" ")
	if e.StepState(0).Running || e.StepElapsed(0) != 3*time.Second {
		t.Fatalf("space must pause at 3s, got %v", e.StepElapsed(0))
	}
	if hs.snd.stops == 0 {
		t.Fatal("pause must stop sounds")
	}
}

func TestStepKeys(t *testing.T) {
	hs := newHarness(t, format.BritishParliamentary(), 80, 24)
	e := hs.m.eng
	hs.press("right", "l", "n", "enter")
	if e.Current() != 4 {
		t.Fatalf("current %d", e.Current())
	}
	hs.press("left", "h")
	if e.Current() != 2 {
		t.Fatalf("current %d", e.Current())
	}
	hs.press("+", "+", "]")
	if e.StepElapsed(2) != 80*time.Second {
		t.Fatalf("adjust: %v", e.StepElapsed(2))
	}
	hs.press("-", "[")
	if e.StepElapsed(2) != 10*time.Second {
		t.Fatalf("adjust: %v", e.StepElapsed(2))
	}
	hs.press("r")
	if e.StepElapsed(2) != 0 {
		t.Fatal("r must reset")
	}
}

func TestSignalsPlayAndFlash(t *testing.T) {
	hs := newHarness(t, format.BritishParliamentary(), 80, 24)
	hs.press("l", " ")
	hs.advance(60*time.Second + 100*time.Millisecond)
	if len(hs.snd.played) != 1 || hs.snd.played[0] != format.SignalSingle {
		t.Fatalf("played %v", hs.snd.played)
	}
	if !hs.m.flashOn() {
		t.Fatal("signal must flash the screen")
	}
	hs.press("b")
	if len(hs.snd.played) != 2 {
		t.Fatal("b must ring the bell")
	}
}

func TestRestartNeedsConfirmation(t *testing.T) {
	hs := newHarness(t, format.BritishParliamentary(), 80, 24)
	e := hs.m.eng
	hs.press("l", " ")
	hs.advance(5 * time.Second)
	hs.press("R", "n")
	if e.Current() != 1 {
		t.Fatal("restart without confirmation")
	}
	hs.press("R")
	if !strings.Contains(hs.m.View(), "Restartovat") {
		t.Fatal("confirmation not shown")
	}
	hs.press("y")
	if e.Current() != 0 || e.AnyRunning() || e.StepElapsed(1) != 0 {
		t.Fatal("restart not done")
	}
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestQuit(t *testing.T) {
	hs := newHarness(t, format.BritishParliamentary(), 80, 24)
	if !isQuit(hs.press("q")) {
		t.Fatal("q must quit when nothing runs")
	}
	hs = newHarness(t, format.BritishParliamentary(), 80, 24)
	hs.press(" ")
	if isQuit(hs.press("q")) {
		t.Fatal("q must ask when a timer runs")
	}
	if isQuit(hs.press("n")) || !hs.m.eng.AnyRunning() {
		t.Fatal("n must cancel")
	}
	hs.press("q")
	if !isQuit(hs.press("y")) {
		t.Fatal("y must quit")
	}
	if !isQuit(hs.press("ctrl+c")) {
		t.Fatal("ctrl+c always quits")
	}
}

func TestQuestionKeys(t *testing.T) {
	hs := newHarness(t, format.Snemovni(), 80, 24)
	hs.press("l", "l") // questioning
	hs.press("up", "up", "up", "down")
	if got := hs.m.eng.StepState(2).QuestionCount; got != 2 {
		t.Fatalf("questions %d", got)
	}
	if !strings.Contains(hs.m.View(), "Otázky 2 / 5") {
		t.Fatalf("counter not shown:\n%s", hs.m.View())
	}
	hs.press("enter")
	if hs.m.eng.Current() != 3 {
		t.Fatal("enter must end questioning")
	}
}

func TestPoolKeysAndTeamPrepStep(t *testing.T) {
	hs := newHarness(t, format.KarlPopper(nil), 80, 24)
	e := hs.m.eng
	hs.press("2")
	if !e.PoolState(format.PoolNegative).Running {
		t.Fatal("2 must start the negative pool")
	}
	hs.advance(10 * time.Second)
	if !strings.Contains(hs.m.View(), "Příprava N 4:50 ▶") {
		t.Fatalf("pool line missing:\n%s", hs.m.View())
	}
	hs.press("2", "l") // team preparation A
	view := hs.m.View()
	if strings.Contains(view, "Příprava N 4:50") {
		t.Fatal("team preparation step shows only its own pool")
	}
	if !strings.Contains(view, "Příprava A") {
		t.Fatalf("pool name missing:\n%s", view)
	}
	hs.press(" ")
	if !e.PoolState(format.PoolAffirmative).Running {
		t.Fatal("space on team preparation must start the team's pool")
	}
}

func TestFormatPicker(t *testing.T) {
	hs := newHarness(t, format.BritishParliamentary(), 80, 24)
	hs.press("f")
	if !strings.Contains(hs.m.View(), "Výběr formátu") {
		t.Fatal("format picker not shown")
	}
	hs.press("down", "enter")
	if hs.m.eng.Format().ID != format.IDKarlPopper || hs.m.Settings().Format != format.IDKarlPopper {
		t.Fatal("format not switched")
	}
	if len(hs.saved) == 0 {
		t.Fatal("format choice must be saved")
	}
}

func TestStepList(t *testing.T) {
	hs := newHarness(t, format.KarlPopper(nil), 80, 12)
	hs.press("L")
	for i := 0; i < 15; i++ {
		hs.press("down")
	}
	view := hs.m.View()
	checkFits(t, view, 80, 12)
	if !strings.Contains(view, "› ") {
		t.Fatalf("cursor must stay visible:\n%s", view)
	}
	hs.press("enter")
	if hs.m.eng.Current() != 15 || hs.m.screen != screenMain {
		t.Fatalf("enter must jump, current %d", hs.m.eng.Current())
	}
}

func TestSettingsScreen(t *testing.T) {
	hs := newHarness(t, format.Custom(3, 5), 80, 24)
	var modes []sound.Mode
	hs.m.newSound = func(mode sound.Mode) sound.Player { modes = append(modes, mode); return hs.snd }
	hs.press("s")
	hs.press("right") // language → en
	if !strings.Contains(hs.m.View(), "Settings") {
		t.Fatalf("language not switched:\n%s", hs.m.View())
	}
	hs.press("down", "down", "right") // sound auto → bell
	if len(modes) != 1 || modes[0] != sound.ModeBell {
		t.Fatalf("sound modes %v", modes)
	}
	hs.press("down", "down", "right") // custom speakers 4 → 5 (defaults)
	if n := len(hs.m.eng.Format().Steps); n != 1+hs.m.Settings().CustomSpeakers {
		t.Fatalf("custom format not rebuilt: %d steps", n)
	}
	hs.press("esc")
	if hs.m.screen != screenMain {
		t.Fatal("esc must go back")
	}
}

func TestBigDigits(t *testing.T) {
	lines := renderBig("1:0", 1, 1)
	want := []string{
		" #    ###",
		"##  # # #",
		" #    # #",
		" #  # # #",
		"###   ###",
	}
	for i := range want {
		got := strings.ReplaceAll(lines[i], "█", "#")
		if got != want[i] {
			t.Errorf("row %d: %q, want %q", i, got, want[i])
		}
	}
	if bigWidth("7:00", 2) != 26 || bigWidth("60:00", 4) != 68 {
		t.Errorf("widths %d %d", bigWidth("7:00", 2), bigWidth("60:00", 4))
	}
	if l := renderBig("12", 2, 2); len(l) != 10 {
		t.Errorf("py=2 must double the rows, got %d", len(l))
	}
}

func TestClockText(t *testing.T) {
	cases := map[int]string{0: "0:00", 59: "0:59", 420: "7:00", 3600: "60:00", -15: "+0:15"}
	for in, want := range cases {
		if got := clockText(in); got != want {
			t.Errorf("clockText(%d) = %q, want %q", in, got, want)
		}
	}
	if got := longDuration(4260); got != "1:11:00" {
		t.Errorf("longDuration = %q", got)
	}
}
