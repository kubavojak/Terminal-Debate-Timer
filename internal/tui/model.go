// Package tui is the Bubble Tea user interface.
package tui

import (
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"debtime/internal/engine"
	"debtime/internal/format"
	"debtime/internal/i18n"
	"debtime/internal/sound"
	"debtime/internal/store"
)

// TickInterval is how often the engine is synced.
const TickInterval = 100 * time.Millisecond

type screen int

const (
	screenMain screen = iota
	screenFormats
	screenSteps
	screenPools
	screenSettings
	screenHelp
)

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmQuit
	confirmRestart
)

type tickMsg time.Time

// Config wires the model to the rest of the app.
type Config struct {
	Engine   *engine.Engine
	Settings store.Settings
	Sound    sound.Player
	// NewSound builds a player when the sound mode changes in settings.
	NewSound func(mode sound.Mode) sound.Player
	// SaveSettings persists changed settings (errors are the caller's to log).
	SaveSettings func(store.Settings)
	// FirstRun opens the format picker.
	FirstRun bool
	NoColor  bool
	// Now overrides the wall clock used for blinking (tests).
	Now func() time.Time
}

// Model is the Bubble Tea model.
type Model struct {
	eng      *engine.Engine
	settings store.Settings
	tr       i18n.Translator
	st       styles
	snd      sound.Player
	newSound func(sound.Mode) sound.Player
	save     func(store.Settings)
	clock    func() time.Time

	width, height int
	screen        screen
	cursor        int
	confirm       confirmKind

	now        time.Time
	flashStart time.Time
	flashUntil time.Time
}

// New builds the model.
func New(cfg Config) *Model {
	m := &Model{
		eng:      cfg.Engine,
		settings: cfg.Settings,
		snd:      cfg.Sound,
		newSound: cfg.NewSound,
		save:     cfg.SaveSettings,
		clock:    cfg.Now,
		width:    80,
		height:   24,
	}
	if m.clock == nil {
		m.clock = time.Now
	}
	if m.snd == nil {
		m.snd = sound.New(sound.Options{Mode: sound.ModeOff})
	}
	m.tr = i18n.New(cfg.Settings.Lang)
	m.st = styles{noColor: cfg.NoColor, highContrast: cfg.Settings.HighContrast}
	m.now = m.clock()
	if cfg.FirstRun {
		m.openFormats()
	}
	return m
}

// Settings returns the current settings.
func (m *Model) Settings() store.Settings { return m.settings }

// Init starts the tick loop.
func (m *Model) Init() tea.Cmd { return tick() }

func tick() tea.Cmd {
	return tea.Tick(TickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.now = m.clock()
		m.handle(m.eng.Sync())
		return m, tick()
	case tea.KeyMsg:
		m.now = m.clock()
		return m, m.key(msg.String())
	}
	return m, nil
}

// handle reacts to engine events: sounds and the visual flash.
func (m *Model) handle(evs []engine.Event) {
	for _, ev := range evs {
		switch ev := ev.(type) {
		case engine.SignalEvent:
			m.signal(ev.Signal.Type)
		case engine.CancelSignals:
			m.snd.Stop()
			m.flashUntil = time.Time{}
		}
	}
}

func (m *Model) signal(t format.SignalType) {
	m.snd.Play(t)
	d := 400 * time.Millisecond
	switch t {
	case format.SignalDouble:
		d = time.Second
	case format.SignalContinuous:
		d = sound.ContinuousDuration
	}
	m.flashStart = m.now
	m.flashUntil = m.now.Add(d)
}

// flashOn blinks the box every 250 ms while a signal is flashing.
func (m *Model) flashOn() bool {
	if !m.now.Before(m.flashUntil) {
		return false
	}
	return (m.now.Sub(m.flashStart)/(250*time.Millisecond))%2 == 0
}

// blinkOn drives the blinking overtime clock.
func (m *Model) blinkOn() bool { return (m.now.UnixMilli()/500)%2 == 0 }

func (m *Model) quit() tea.Cmd {
	m.snd.Stop()
	return tea.Quit
}

func (m *Model) key(k string) tea.Cmd {
	if k == "ctrl+c" {
		return m.quit()
	}
	if m.confirm != confirmNone {
		kind := m.confirm
		m.confirm = confirmNone
		if k == "y" || k == "Y" || k == "a" || k == "A" {
			switch kind {
			case confirmQuit:
				return m.quit()
			case confirmRestart:
				m.handle(m.eng.FullRestart())
			}
		}
		return nil
	}
	switch m.screen {
	case screenFormats:
		return m.keyFormats(k)
	case screenSteps:
		return m.keySteps(k)
	case screenPools:
		return m.keyPools(k)
	case screenSettings:
		return m.keySettings(k)
	case screenHelp:
		if k == "esc" || k == "q" || k == "?" || k == "enter" {
			m.screen = screenMain
		}
		return nil
	}
	return m.keyMain(k)
}

func (m *Model) keyMain(k string) tea.Cmd {
	e := m.eng
	switch k {
	case " ", "space":
		m.handle(e.Toggle())
	case "left", "h":
		m.handle(e.Prev())
	case "right", "l", "n", "enter":
		m.handle(e.Next())
	case "+", "=":
		m.handle(e.AdjustTime(10 * time.Second))
	case "-", "_":
		m.handle(e.AdjustTime(-10 * time.Second))
	case "]":
		m.handle(e.AdjustTime(time.Minute))
	case "[":
		m.handle(e.AdjustTime(-time.Minute))
	case "r":
		m.handle(e.Reset())
	case "R":
		m.confirm = confirmRestart
	case "b":
		m.signal(format.SignalSingle)
	case "1", "2":
		m.handle(e.TogglePrepPool(int(k[0] - '1')))
	case "up", "k":
		m.handle(e.ChangeQuestions(1))
	case "down", "j":
		m.handle(e.ChangeQuestions(-1))
	case "u":
		m.settings.CountUp = !m.settings.CountUp
		m.saveSettings()
	case "f":
		m.openFormats()
	case "L":
		m.screen = screenSteps
		m.cursor = e.Current()
	case "p":
		m.screen = screenPools
		m.cursor = 0
	case "s":
		m.screen = screenSettings
		m.cursor = 0
	case "?":
		m.screen = screenHelp
	case "q":
		if e.AnyRunning() {
			m.confirm = confirmQuit
			return nil
		}
		return m.quit()
	}
	return nil
}

func (m *Model) openFormats() {
	m.screen = screenFormats
	m.cursor = 0
	for i, id := range format.IDs() {
		if id == m.eng.Format().ID {
			m.cursor = i
		}
	}
}

func moveCursor(cursor, n int, k string) int {
	switch k {
	case "up", "k":
		cursor--
	case "down", "j":
		cursor++
	case "home", "g":
		cursor = 0
	case "end", "G":
		cursor = n - 1
	case "pgup":
		cursor -= 10
	case "pgdown":
		cursor += 10
	}
	if cursor >= n {
		cursor = n - 1
	}
	if cursor < 0 {
		cursor = 0
	}
	return cursor
}

func (m *Model) keyFormats(k string) tea.Cmd {
	ids := format.IDs()
	switch k {
	case "esc", "q":
		m.screen = screenMain
	case "enter", " ", "space":
		m.selectFormat(ids[m.cursor])
		m.screen = screenMain
	default:
		m.cursor = moveCursor(m.cursor, len(ids), k)
	}
	return nil
}

func (m *Model) selectFormat(id string) {
	f, ok := format.ByID(id, m.settings.FormatOptions())
	if !ok {
		return
	}
	m.settings.Format = id
	m.saveSettings()
	if f.Signature() != m.eng.Format().Signature() {
		_, evs := m.eng.SetFormat(f)
		m.handle(evs)
	}
}

// rebuildFormat applies changed format options to the running format.
func (m *Model) rebuildFormat() {
	f, ok := format.ByID(m.eng.Format().ID, m.settings.FormatOptions())
	if !ok {
		return
	}
	_, evs := m.eng.SetFormat(f)
	m.handle(evs)
}

func (m *Model) keySteps(k string) tea.Cmd {
	n := len(m.eng.Format().Steps)
	switch k {
	case "esc", "q", "L":
		m.screen = screenMain
	case "enter":
		m.handle(m.eng.GoTo(m.cursor))
		m.screen = screenMain
	default:
		m.cursor = moveCursor(m.cursor, n, k)
	}
	return nil
}

func (m *Model) keyPools(k string) tea.Cmd {
	n := len(m.eng.Format().PrepPools)
	switch k {
	case "esc", "q", "p":
		m.screen = screenMain
	case " ", "space", "enter":
		m.handle(m.eng.TogglePrepPool(m.cursor))
	case "r":
		m.handle(m.eng.ResetPrepPool(m.cursor))
	case "1", "2":
		m.handle(m.eng.TogglePrepPool(int(k[0] - '1')))
	default:
		if n > 0 {
			m.cursor = moveCursor(m.cursor, n, k)
		}
	}
	return nil
}

func (m *Model) keySettings(k string) tea.Cmd {
	items := m.settingItems()
	switch k {
	case "esc", "q", "s":
		m.screen = screenMain
	case "left", "h", "-":
		items[m.cursor].change(-1)
	case "right", "l", "+", "enter", " ", "space":
		items[m.cursor].change(1)
	default:
		m.cursor = moveCursor(m.cursor, len(items), k)
	}
	return nil
}

func (m *Model) saveSettings() {
	if m.save != nil {
		m.save(m.settings)
	}
}

type settingItem struct {
	label  string
	value  string
	change func(delta int)
}

func cycle(options []string, current string, delta int) string {
	idx := 0
	for i, o := range options {
		if o == current {
			idx = i
		}
	}
	idx = (idx + delta + len(options)) % len(options)
	return options[idx]
}

func (m *Model) onOff(v bool) string {
	if v {
		return m.tr.T("on")
	}
	return m.tr.T("off")
}

func (m *Model) settingItems() []settingItem {
	s := &m.settings
	tr := m.tr
	counting := tr.T("countDown")
	if s.CountUp {
		counting = tr.T("countUp")
	}
	soundNames := map[string]string{"auto": tr.T("soundAuto"), "bell": tr.T("soundBell"), "off": tr.T("soundOff")}

	items := []settingItem{
		{tr.T("settingLanguage"), tr.T("langName"), func(d int) {
			s.Lang = cycle(i18n.Supported(), tr.Lang(), d)
			m.tr = i18n.New(s.Lang)
			m.saveSettings()
		}},
		{tr.T("settingCounting"), counting, func(int) {
			s.CountUp = !s.CountUp
			m.saveSettings()
		}},
		{tr.T("settingSound"), soundNames[s.Sound], func(d int) {
			s.Sound = cycle([]string{"auto", "bell", "off"}, s.Sound, d)
			m.saveSettings()
			if m.newSound != nil {
				m.snd.Close()
				m.snd = m.newSound(sound.Mode(s.Sound))
			}
		}},
		{tr.T("settingHighContrast"), m.onOff(s.HighContrast), func(int) {
			s.HighContrast = !s.HighContrast
			m.st.highContrast = s.HighContrast
			m.saveSettings()
		}},
		{tr.T("settingCustomSpeakers"), strconv.Itoa(s.CustomSpeakers), func(d int) {
			s.CustomSpeakers = clampInt(s.CustomSpeakers+d, format.CustomMinSpeakers, format.CustomMaxSpeakers)
			m.saveSettings()
			m.rebuildFormatIf(format.IDCustom)
		}},
		{tr.T("settingCustomMinutes"), strconv.Itoa(s.CustomMinutes), func(d int) {
			s.CustomMinutes = clampInt(s.CustomMinutes+d, format.CustomMinMinutes, format.CustomMaxMinutes)
			m.saveSettings()
			m.rebuildFormatIf(format.IDCustom)
		}},
		{tr.T("settingConsultation"), m.onOff(s.Consultation), func(int) {
			s.Consultation = !s.Consultation
			m.saveSettings()
			m.rebuildFormatIf(format.IDResitelska)
		}},
	}
	for _, ans := range format.KPAnswerers {
		items = append(items, settingItem{tr.F("settingKPAsker", ans), s.KPAskers[ans], func(d int) {
			askers := map[string]string{}
			for k, v := range s.KPAskers {
				askers[k] = v
			}
			askers[ans] = cycle(format.ValidKPAskers(ans), askers[ans], d)
			s.KPAskers = askers
			m.saveSettings()
			m.rebuildFormatIf(format.IDKarlPopper)
		}})
	}
	return items
}

func (m *Model) rebuildFormatIf(id string) {
	if m.eng.Format().ID == id {
		m.rebuildFormat()
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
