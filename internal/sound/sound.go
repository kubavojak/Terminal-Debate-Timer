// Package sound plays the timer signals. Layers, in order of preference:
// bundled WAV files through pw-play / paplay / aplay, the terminal bell
// (BEL), nothing. Playback is asynchronous and never blocks the UI.
package sound

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"debtime/assets"
	"debtime/internal/format"
)

// Mode selects the sound layer.
type Mode string

const (
	ModeAuto Mode = "auto"
	ModeBell Mode = "bell"
	ModeOff  Mode = "off"
)

// ParseMode validates a mode name.
func ParseMode(s string) (Mode, bool) {
	switch Mode(s) {
	case ModeAuto, ModeBell, ModeOff:
		return Mode(s), true
	}
	return "", false
}

// Player plays signals.
type Player interface {
	// Play starts a signal, replacing one that is still playing.
	Play(t format.SignalType)
	// Stop cancels the signal that is playing (the continuous beep).
	Stop()
	// Close stops playback and waits briefly for it to end.
	Close()
	// Backend names the active layer, for --debug output.
	Backend() string
}

// ContinuousDuration is how long the continuous beep lasts.
const ContinuousDuration = format.ContinuousBeepSec * time.Second

// Bell timings.
const (
	DoubleBellGap     = 200 * time.Millisecond
	ContinuousBellGap = 300 * time.Millisecond
)

type backend interface {
	play(ctx context.Context, t format.SignalType) error
	name() string
}

type runFunc func(ctx context.Context, name string, args ...string) error

type lookPathFunc func(string) (string, error)

// candidate audio players in order of preference.
var candidates = []struct {
	bin  string
	args []string
}{
	{"pw-play", nil},
	{"paplay", nil},
	{"aplay", []string{"-q"}},
	{"afplay", nil}, // macOS
}

var files = map[format.SignalType]string{
	format.SignalSingle:     "single_beep.wav",
	format.SignalDouble:     "double_beep.wav",
	format.SignalContinuous: "continuous_beep.wav",
}

// Options configures New.
type Options struct {
	Mode     Mode
	CacheDir string      // where the WAV files are unpacked
	Bell     io.Writer   // where BEL is written; default /dev/tty or stderr
	Logger   *log.Logger // nil discards errors
}

// New builds a player for the mode. It never fails: when a layer is not
// available the next one is used.
func New(opt Options) Player {
	logger := opt.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	bellOut := opt.Bell
	if bellOut == nil && opt.Mode != ModeOff {
		bellOut = openTTY()
	}
	b := selectBackend(opt.Mode, opt.CacheDir, bellOut, exec.LookPath, runCommand, logger)
	logger.Printf("sound: using %s", b.name())
	return newPlayer(b, logger)
}

func selectBackend(mode Mode, cacheDir string, bellOut io.Writer, lookPath lookPathFunc, run runFunc, logger *log.Logger) backend {
	bell := &bellBackend{w: bellOut}
	switch mode {
	case ModeOff:
		return nopBackend{}
	case ModeBell:
		return bell
	}
	for _, c := range candidates {
		path, err := lookPath(c.bin)
		if err != nil {
			continue
		}
		paths, err := extract(cacheDir)
		if err != nil {
			logger.Printf("sound: cannot unpack sounds: %v", err)
			return bell
		}
		return &fallbackBackend{
			primary:  &commandBackend{bin: path, args: c.args, files: paths, run: run},
			fallback: bell,
			logger:   logger,
		}
	}
	return bell
}

// extract unpacks the bundled WAV files into dir/sounds.
func extract(dir string) (map[format.SignalType]string, error) {
	if dir == "" {
		return nil, errors.New("no cache directory")
	}
	dir = filepath.Join(dir, "sounds")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	out := map[format.SignalType]string{}
	for t, name := range files {
		data, err := fs.ReadFile(assets.Sounds, "sounds/"+name)
		if err != nil {
			return nil, err
		}
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err != nil || st.Size() != int64(len(data)) {
			if err := os.WriteFile(p, data, 0o644); err != nil {
				return nil, err
			}
		}
		out[t] = p
	}
	return out, nil
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	// Keep the player's output away from the TUI.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	return cmd.Run()
}

func openTTY() io.Writer {
	if f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		return f
	}
	return os.Stderr
}

type player struct {
	mu      sync.Mutex
	backend backend
	logger  *log.Logger
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newPlayer(b backend, logger *log.Logger) *player {
	return &player{backend: b, logger: logger}
}

func (p *player) Backend() string { return p.backend.name() }

func (p *player) Play(t format.SignalType) {
	if t == format.SignalNone {
		return
	}
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.mu.Unlock()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				p.logger.Printf("sound: panic: %v", r)
			}
		}()
		if err := p.backend.play(ctx, t); err != nil && ctx.Err() == nil {
			p.logger.Printf("sound: %s %s: %v", p.backend.name(), t, err)
		}
	}()
}

func (p *player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

func (p *player) Close() {
	p.Stop()
	done := make(chan struct{})
	go func() { p.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}
}

type nopBackend struct{}

func (nopBackend) play(context.Context, format.SignalType) error { return nil }
func (nopBackend) name() string                                  { return "off" }

// commandBackend plays a WAV file with an external player. Cancelling the
// context kills the player process.
type commandBackend struct {
	bin   string
	args  []string
	files map[format.SignalType]string
	run   runFunc
}

func (c *commandBackend) name() string { return filepath.Base(c.bin) }

func (c *commandBackend) play(ctx context.Context, t format.SignalType) error {
	f, ok := c.files[t]
	if !ok {
		return fmt.Errorf("no sound for %s", t)
	}
	return c.run(ctx, c.bin, append(append([]string(nil), c.args...), f)...)
}

// fallbackBackend switches to the bell for good once the player fails
// (no audio server, SSH session, ...).
type fallbackBackend struct {
	mu       sync.Mutex
	primary  backend
	fallback backend
	failed   bool
	logger   *log.Logger
}

func (f *fallbackBackend) name() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failed {
		return f.fallback.name()
	}
	return f.primary.name()
}

func (f *fallbackBackend) play(ctx context.Context, t format.SignalType) error {
	f.mu.Lock()
	failed := f.failed
	f.mu.Unlock()
	if !failed {
		err := f.primary.play(ctx, t)
		if err == nil || ctx.Err() != nil {
			return nil
		}
		f.logger.Printf("sound: %s failed (%v), switching to %s", f.primary.name(), err, f.fallback.name())
		f.mu.Lock()
		f.failed = true
		f.mu.Unlock()
	}
	return f.fallback.play(ctx, t)
}

// bellBackend rings the terminal bell. It works over SSH (the client's
// terminal rings) but may be muted in the terminal settings.
type bellBackend struct {
	mu    sync.Mutex
	w     io.Writer
	sleep func(ctx context.Context, d time.Duration) bool
}

func (b *bellBackend) name() string { return "bell" }

func (b *bellBackend) ring() error {
	if b.w == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	_, err := b.w.Write([]byte{'\a'})
	return err
}

func (b *bellBackend) wait(ctx context.Context, d time.Duration) bool {
	if b.sleep != nil {
		return b.sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (b *bellBackend) play(ctx context.Context, t format.SignalType) error {
	switch t {
	case format.SignalSingle:
		return b.ring()
	case format.SignalDouble:
		if err := b.ring(); err != nil {
			return err
		}
		if !b.wait(ctx, DoubleBellGap) {
			return nil
		}
		return b.ring()
	case format.SignalContinuous:
		for elapsed := time.Duration(0); elapsed < ContinuousDuration; elapsed += ContinuousBellGap {
			if ctx.Err() != nil {
				return nil
			}
			if err := b.ring(); err != nil {
				return err
			}
			if !b.wait(ctx, ContinuousBellGap) {
				return nil
			}
		}
	}
	return nil
}
