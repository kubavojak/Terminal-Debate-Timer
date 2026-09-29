// Command debtime is a debate timer for the terminal.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"debtime/internal/clock"
	"debtime/internal/engine"
	"debtime/internal/format"
	"debtime/internal/i18n"
	"debtime/internal/sound"
	"debtime/internal/store"
	"debtime/internal/tui"
)

// version is set by the release build (-ldflags "-X main.version=...").
var version = "dev"

type options struct {
	format       string
	speakers     int
	minutes      int
	consultation bool
	lang         string
	sound        string
	noRestore    bool
	highContrast bool
	listFormats  bool
	version      bool
	debug        bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var o options
	fl := flag.NewFlagSet("debtime", flag.ContinueOnError)
	fl.SetOutput(stderr)
	fl.StringVar(&o.format, "format", "", "format: "+strings.Join(format.IDs(), "|"))
	fl.IntVar(&o.speakers, "speakers", 0, fmt.Sprintf("number of speeches for custom (%d–%d)", format.CustomMinSpeakers, format.CustomMaxSpeakers))
	fl.IntVar(&o.minutes, "minutes", 0, fmt.Sprintf("minutes per speech for custom (%d–%d)", format.CustomMinMinutes, format.CustomMaxMinutes))
	fl.BoolVar(&o.consultation, "consultation", false, "15 s consultation between parts of Řešitelská")
	fl.StringVar(&o.lang, "lang", "", "language: cs|en (default from $LANG, otherwise cs)")
	fl.StringVar(&o.sound, "sound", "", "sound: auto|bell|off (default auto)")
	fl.BoolVar(&o.noRestore, "no-restore", false, "ignore the saved timer state")
	fl.BoolVar(&o.highContrast, "high-contrast", false, "high contrast colours")
	fl.BoolVar(&o.listFormats, "list-formats", false, "list formats and exit")
	fl.BoolVar(&o.version, "version", false, "print the version and exit")
	fl.BoolVar(&o.debug, "debug", false, "write a debug log to the state directory")
	fl.Usage = func() {
		fmt.Fprintf(stderr, "Usage:\n  debtime [flags]\n  debtime [flags] plan <format> [flags]\n\nFlags:\n")
		fl.PrintDefaults()
	}
	// parse returns the exit code for a flag error, or -1 to continue.
	parse := func(a []string) int {
		if err := fl.Parse(a); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		return -1
	}
	if code := parse(args); code >= 0 {
		return code
	}
	// "plan <format>" may stand between flags; flags after it are parsed too.
	planFormat := ""
	if rest := fl.Args(); len(rest) > 0 {
		if rest[0] != "plan" {
			fmt.Fprintf(stderr, "debtime: unexpected argument %q\n", rest[0])
			fl.Usage()
			return 2
		}
		if len(rest) < 2 {
			fmt.Fprintf(stderr, "Usage: debtime plan <%s>\n", strings.Join(format.IDs(), "|"))
			return 2
		}
		planFormat = rest[1]
		if code := parse(rest[2:]); code >= 0 {
			return code
		}
		if len(fl.Args()) > 0 {
			fmt.Fprintf(stderr, "debtime: unexpected argument %q\n", fl.Args()[0])
			return 2
		}
	}
	set := map[string]bool{}
	fl.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if o.version {
		fmt.Fprintf(stdout, "debtime %s\n", version)
		return 0
	}

	st := store.New()
	settings, found, settingsErr := st.LoadSettings()

	if set["lang"] {
		if o.lang != "cs" && o.lang != "en" {
			fmt.Fprintf(stderr, "debtime: unknown language %q (cs|en)\n", o.lang)
			return 2
		}
		settings.Lang = o.lang
	}
	if settings.Lang == "" {
		settings.Lang = i18n.Detect(os.Getenv)
	}
	tr := i18n.New(settings.Lang)

	if set["format"] {
		if _, ok := format.ByID(o.format, settings.FormatOptions()); !ok {
			fmt.Fprintln(stderr, "debtime: "+tr.F("cliUnknownFormat", o.format))
			return 2
		}
		settings.Format = o.format
	}
	if set["speakers"] {
		settings.CustomSpeakers = o.speakers
	}
	if set["minutes"] {
		settings.CustomMinutes = o.minutes
	}
	if set["consultation"] {
		settings.Consultation = o.consultation
	}
	if set["sound"] {
		if _, ok := sound.ParseMode(o.sound); !ok {
			fmt.Fprintf(stderr, "debtime: unknown sound mode %q (auto|bell|off)\n", o.sound)
			return 2
		}
		settings.Sound = o.sound
	}
	if set["high-contrast"] {
		settings.HighContrast = o.highContrast
	}
	settings.Normalize()
	if settings.Lang == "" {
		settings.Lang = tr.Lang()
	}

	if o.listFormats {
		listFormats(stdout, tr, settings.FormatOptions())
		return 0
	}
	if planFormat != "" {
		f, ok := format.ByID(planFormat, settings.FormatOptions())
		if !ok {
			fmt.Fprintln(stderr, "debtime: "+tr.F("cliUnknownFormat", planFormat))
			return 2
		}
		printPlan(stdout, tr, f)
		return 0
	}

	logger := log.New(io.Discard, "", 0)
	if o.debug {
		if err := os.MkdirAll(st.StateDir, 0o755); err == nil {
			if f, err := os.OpenFile(filepath.Join(st.StateDir, "debug.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
				defer f.Close()
				logger = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
			}
		}
	}
	if settingsErr != nil {
		logger.Printf("settings: %v (using defaults)", settingsErr)
	}

	firstRun := !found && settings.Format == ""
	formatID := settings.Format
	if formatID == "" {
		formatID = format.IDBritishParliamentary
	}
	f, _ := format.ByID(formatID, settings.FormatOptions())

	eng := engine.New(clock.NewReal(), f)
	if !o.noRestore {
		if snap, err := st.LoadState(); err == nil {
			if err := eng.Restore(snap); err != nil {
				logger.Printf("state: not restored: %v", err)
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			logger.Printf("state: %v", err)
		}
	}
	eng.OnChange = func(e *engine.Engine) {
		if err := st.SaveState(e.Snapshot()); err != nil {
			logger.Printf("state: save: %v", err)
		}
	}

	newSound := func(mode sound.Mode) sound.Player {
		return sound.New(sound.Options{Mode: mode, CacheDir: store.CacheDir(), Logger: logger})
	}
	player := newSound(sound.Mode(settings.Sound))

	model := tui.New(tui.Config{
		Engine:   eng,
		Settings: settings,
		Sound:    player,
		NewSound: newSound,
		SaveSettings: func(s store.Settings) {
			if err := st.SaveSettings(s); err != nil {
				logger.Printf("settings: save: %v", err)
			}
		},
		FirstRun: firstRun,
		NoColor:  os.Getenv("NO_COLOR") != "",
	})

	// Bubble Tea restores the terminal on quit, Ctrl+C, SIGTERM and on a
	// panic inside Update/View.
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, runErr := p.Run()

	player.Close()
	if err := st.SaveState(eng.Snapshot()); err != nil {
		logger.Printf("state: save: %v", err)
	}
	if runErr != nil && !errors.Is(runErr, tea.ErrProgramKilled) && !errors.Is(runErr, tea.ErrInterrupted) {
		fmt.Fprintf(stderr, "debtime: %v\n", runErr)
		return 1
	}
	return 0
}

func listFormats(w io.Writer, tr i18n.Translator, opt format.Options) {
	for _, f := range format.All(opt) {
		name := tr.FormatName(f)
		fmt.Fprintf(w, "%-11s %s%s%s\n", f.ID, name, strings.Repeat(" ", max(24-displayWidth(name), 1)), longDuration(f.TotalSec()))
	}
}

func printPlan(w io.Writer, tr i18n.Translator, f format.Format) {
	fmt.Fprintln(w, tr.FormatName(f))
	n := 0
	for _, s := range f.Steps {
		name := tr.StepName(s)
		if s.Type == format.TeamPreparation {
			fmt.Fprintf(w, "      · %s\n", name)
			continue
		}
		n++
		line := fmt.Sprintf("%3d.  %s%s%6s", n, name, strings.Repeat(" ", max(34-displayWidth(name), 1)), clockText(s.DurationSec))
		var extra []string
		if s.Poi != nil {
			extra = append(extra, tr.F("cliPoi", clockText(s.Poi.FromSec), clockText(s.Poi.ToSec)))
		}
		extra = append(extra, signalsText(s.Signals))
		fmt.Fprintf(w, "%s   %s\n", line, strings.Join(extra, "  "))
	}
	if len(f.PrepPools) > 0 {
		fmt.Fprintf(w, "%s:\n", tr.T("cliPools"))
		for _, p := range f.PrepPools {
			name := tr.PoolName(p)
			fmt.Fprintf(w, "      %s%s%6s   %s\n", name, strings.Repeat(" ", max(34-displayWidth(name), 1)), clockText(p.DurationSec), signalsText(p.Signals))
		}
	}
	fmt.Fprintf(w, "%s: %s\n", tr.T("cliTotal"), longDuration(f.TotalSec()))
}

// signalsText shows signals as time and dots: • single, •• double, ••• continuous.
func signalsText(sigs []format.Signal) string {
	var parts []string
	for _, s := range sigs {
		parts = append(parts, clockText(s.AtSec)+" "+strings.Repeat("•", int(s.Type)))
	}
	return strings.Join(parts, "  ")
}

func displayWidth(s string) int { return len([]rune(s)) }

func clockText(sec int) string { return fmt.Sprintf("%d:%02d", sec/60, sec%60) }

func longDuration(sec int) string {
	if sec >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", sec/3600, sec/60%60, sec%60)
	}
	return clockText(sec)
}
