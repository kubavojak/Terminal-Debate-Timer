// Package store saves settings and timer state as JSON in XDG directories.
// Writes are atomic and a damaged file falls back to defaults.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"debtime/internal/engine"
	"debtime/internal/format"
)

const appName = "debtime"

func xdgDir(env string, fallback ...string) string {
	if v := os.Getenv(env); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, appName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(append(append([]string{home}, fallback...), appName)...)
}

// ConfigDir is $XDG_CONFIG_HOME/debtime (default ~/.config/debtime).
func ConfigDir() string { return xdgDir("XDG_CONFIG_HOME", ".config") }

// StateDir is $XDG_STATE_HOME/debtime (default ~/.local/state/debtime).
func StateDir() string { return xdgDir("XDG_STATE_HOME", ".local", "state") }

// CacheDir is $XDG_CACHE_HOME/debtime (default ~/.cache/debtime).
func CacheDir() string { return xdgDir("XDG_CACHE_HOME", ".cache") }

// Settings are the user preferences.
type Settings struct {
	Format         string            `json:"format"`
	CustomSpeakers int               `json:"customSpeakers"`
	CustomMinutes  int               `json:"customMinutes"`
	Consultation   bool              `json:"consultation"`
	KPAskers       map[string]string `json:"kpAskers,omitempty"`
	Lang           string            `json:"lang,omitempty"`
	CountUp        bool              `json:"countUp"`
	Sound          string            `json:"sound"`
	HighContrast   bool              `json:"highContrast"`
}

// DefaultSettings returns the settings used on the first start.
func DefaultSettings() Settings {
	return Settings{
		CustomSpeakers: format.DefaultCustomSpeakers,
		CustomMinutes:  format.DefaultCustomMinutes,
		KPAskers:       format.DefaultKPAskers(),
		Sound:          "auto",
	}
}

// Normalize replaces invalid values with defaults.
func (s *Settings) Normalize() {
	if _, ok := format.ByID(s.Format, format.DefaultOptions()); !ok {
		s.Format = ""
	}
	s.CustomSpeakers = clamp(s.CustomSpeakers, format.CustomMinSpeakers, format.CustomMaxSpeakers, format.DefaultCustomSpeakers)
	s.CustomMinutes = clamp(s.CustomMinutes, format.CustomMinMinutes, format.CustomMaxMinutes, format.DefaultCustomMinutes)
	s.KPAskers = format.NormalizeKPAskers(s.KPAskers)
	if s.Lang != "cs" && s.Lang != "en" {
		s.Lang = ""
	}
	switch s.Sound {
	case "auto", "bell", "off":
	default:
		s.Sound = "auto"
	}
}

func clamp(v, lo, hi, def int) int {
	if v == 0 {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// FormatOptions converts the settings to format options.
func (s Settings) FormatOptions() format.Options {
	return format.Options{
		CustomSpeakers: s.CustomSpeakers,
		CustomMinutes:  s.CustomMinutes,
		Consultation:   s.Consultation,
		KPAskers:       s.KPAskers,
	}
}

// Store knows where the files live.
type Store struct {
	ConfigDir string
	StateDir  string
}

// New returns a store using the XDG directories.
func New() *Store { return &Store{ConfigDir: ConfigDir(), StateDir: StateDir()} }

// SettingsPath is the settings file.
func (s *Store) SettingsPath() string { return filepath.Join(s.ConfigDir, "settings.json") }

// StatePath is the timer state file.
func (s *Store) StatePath() string { return filepath.Join(s.StateDir, "timer_state.json") }

// LoadSettings reads the settings. found is false when no file exists yet
// (first start). A damaged file gives defaults and an error to log.
func (s *Store) LoadSettings() (settings Settings, found bool, err error) {
	settings = DefaultSettings()
	data, err := os.ReadFile(s.SettingsPath())
	if errors.Is(err, fs.ErrNotExist) {
		return settings, false, nil
	}
	if err != nil {
		return settings, false, err
	}
	loaded := DefaultSettings()
	if err := json.Unmarshal(data, &loaded); err != nil {
		return settings, true, fmt.Errorf("settings %s: %w", s.SettingsPath(), err)
	}
	loaded.Normalize()
	return loaded, true, nil
}

// SaveSettings writes the settings atomically.
func (s *Store) SaveSettings(settings Settings) error {
	return writeJSON(s.SettingsPath(), settings)
}

// LoadState reads the saved timer state.
func (s *Store) LoadState() (engine.Snapshot, error) {
	var snap engine.Snapshot
	data, err := os.ReadFile(s.StatePath())
	if err != nil {
		return snap, err
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return engine.Snapshot{}, fmt.Errorf("state %s: %w", s.StatePath(), err)
	}
	return snap, nil
}

// SaveState writes the timer state atomically.
func (s *Store) SaveState(snap engine.Snapshot) error {
	return writeJSON(s.StatePath(), snap)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, append(data, '\n'), 0o644)
}

// WriteFileAtomic writes to a temporary file in the same directory and
// renames it over the target, so readers never see a half-written file.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	cleanup := func(err error) error {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(err)
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(err)
	}
	_ = tmp.Chmod(perm) // best effort, not supported everywhere
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
