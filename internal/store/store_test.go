package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"debtime/internal/clock"
	"debtime/internal/engine"
	"debtime/internal/format"
)

func newStore(t *testing.T) *Store {
	dir := t.TempDir()
	return &Store{ConfigDir: filepath.Join(dir, "config"), StateDir: filepath.Join(dir, "state")}
}

func TestSettingsFirstStart(t *testing.T) {
	s := newStore(t)
	got, found, err := s.LoadSettings()
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if !reflect.DeepEqual(got, DefaultSettings()) {
		t.Fatalf("got %+v", got)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	s := newStore(t)
	want := DefaultSettings()
	want.Format = format.IDKarlPopper
	want.CustomSpeakers = 6
	want.CustomMinutes = 3
	want.Consultation = true
	want.KPAskers = map[string]string{"A1": "N1", "N1": "A2", "A2": "N2", "N2": "A3"}
	want.Lang = "en"
	want.CountUp = true
	want.Sound = "bell"
	want.HighContrast = true
	if err := s.SaveSettings(want); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.LoadSettings()
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestSettingsCorrupt(t *testing.T) {
	s := newStore(t)
	os.MkdirAll(s.ConfigDir, 0o755)
	os.WriteFile(s.SettingsPath(), []byte("{not json"), 0o644)
	got, found, err := s.LoadSettings()
	if err == nil || !found {
		t.Fatal("corrupt file must report an error")
	}
	if !reflect.DeepEqual(got, DefaultSettings()) {
		t.Fatalf("corrupt file must give defaults, got %+v", got)
	}
}

func TestSettingsNormalize(t *testing.T) {
	s := newStore(t)
	os.MkdirAll(s.ConfigDir, 0o755)
	os.WriteFile(s.SettingsPath(), []byte(`{"format":"xx","customSpeakers":500,"customMinutes":-3,"lang":"de","sound":"loud","kpAskers":{"A1":"A2"}}`), 0o644)
	got, _, err := s.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != "" || got.CustomSpeakers != 20 || got.CustomMinutes != 1 || got.Lang != "" || got.Sound != "auto" {
		t.Fatalf("got %+v", got)
	}
	if got.KPAskers["A1"] != "N3" {
		t.Fatalf("invalid asker must fall back, got %v", got.KPAskers)
	}
}

func TestStateRoundTrip(t *testing.T) {
	s := newStore(t)
	c := clock.NewFake(time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC))
	e := engine.New(c, format.KarlPopper(nil))
	e.GoTo(2)
	e.Start()
	c.Advance(42 * time.Second)
	if err := s.SaveState(e.Snapshot()); err != nil {
		t.Fatal(err)
	}
	snap, err := s.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	e2 := engine.New(c, format.KarlPopper(nil))
	if err := e2.Restore(snap); err != nil {
		t.Fatal(err)
	}
	if e2.Current() != 2 || e2.StepElapsed(2) != 42*time.Second || !e2.StepState(2).Running {
		t.Fatalf("restored %d %v", e2.Current(), e2.StepElapsed(2))
	}
}

func TestStateCorrupt(t *testing.T) {
	s := newStore(t)
	os.MkdirAll(s.StateDir, 0o755)
	os.WriteFile(s.StatePath(), []byte("garbage"), 0o644)
	if _, err := s.LoadState(); err == nil {
		t.Fatal("expected an error")
	}
}

func TestWriteFileAtomicReplaces(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "f.json")
	if err := WriteFileAtomic(p, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if string(data) != "two" {
		t.Fatalf("got %q", data)
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
}

func TestXDGDirs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "cfg"))
	if got := ConfigDir(); filepath.Base(got) != "debtime" || filepath.Base(filepath.Dir(got)) != "cfg" {
		t.Fatalf("got %s", got)
	}
	t.Setenv("XDG_STATE_HOME", "relative/path") // relative paths are ignored
	if got := StateDir(); !filepath.IsAbs(got) && filepath.Base(filepath.Dir(got)) != "state" {
		t.Fatalf("got %s", got)
	}
}
