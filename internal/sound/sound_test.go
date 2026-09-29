package sound

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"debtime/internal/format"
)

var discard = log.New(io.Discard, "", 0)

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func lookPathOnly(available ...string) lookPathFunc {
	return func(bin string) (string, error) {
		for _, a := range available {
			if a == bin {
				return "/usr/bin/" + bin, nil
			}
		}
		return "", errors.New("not found")
	}
}

func noRun(context.Context, string, ...string) error { return nil }

func TestFallbackOrder(t *testing.T) {
	cache := t.TempDir()
	cases := []struct {
		available []string
		want      string
	}{
		{[]string{"aplay", "paplay", "pw-play"}, "pw-play"},
		{[]string{"aplay", "paplay"}, "paplay"},
		{[]string{"aplay"}, "aplay"},
		{nil, "bell"},
	}
	for _, c := range cases {
		b := selectBackend(ModeAuto, cache, io.Discard, lookPathOnly(c.available...), noRun, discard)
		if b.name() != c.want {
			t.Errorf("%v: got %s, want %s", c.available, b.name(), c.want)
		}
	}
	if b := selectBackend(ModeBell, cache, io.Discard, lookPathOnly("pw-play"), noRun, discard); b.name() != "bell" {
		t.Errorf("bell mode: %s", b.name())
	}
	if b := selectBackend(ModeOff, cache, io.Discard, lookPathOnly("pw-play"), noRun, discard); b.name() != "off" {
		t.Errorf("off mode: %s", b.name())
	}
}

func TestExtractWritesFiles(t *testing.T) {
	cache := t.TempDir()
	paths, err := extract(cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []format.SignalType{format.SignalSingle, format.SignalDouble, format.SignalContinuous} {
		p := paths[typ]
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
			t.Errorf("%s is not a WAV file", filepath.Base(p))
		}
	}
}

func TestCommandBackendArgs(t *testing.T) {
	var got []string
	run := func(_ context.Context, name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	}
	b := selectBackend(ModeAuto, t.TempDir(), io.Discard, lookPathOnly("aplay"), run, discard)
	if err := b.play(context.Background(), format.SignalDouble); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "/usr/bin/aplay" || got[1] != "-q" || filepath.Base(got[2]) != "double_beep.wav" {
		t.Fatalf("got %v", got)
	}
}

func TestPlayerFailureFallsBackToBell(t *testing.T) {
	bell := &safeBuffer{}
	calls := 0
	run := func(context.Context, string, ...string) error {
		calls++
		return errors.New("no audio server")
	}
	b := selectBackend(ModeAuto, t.TempDir(), bell, lookPathOnly("paplay"), run, discard)
	b.play(context.Background(), format.SignalSingle)
	b.play(context.Background(), format.SignalSingle)
	if calls != 1 {
		t.Errorf("a failed player must not be retried, called %d times", calls)
	}
	if bell.String() != "\a\a" || b.name() != "bell" {
		t.Errorf("bell output %q, backend %s", bell.String(), b.name())
	}
}

func instantBell(w io.Writer) (*bellBackend, *[]time.Duration) {
	var waits []time.Duration
	b := &bellBackend{w: w}
	b.sleep = func(ctx context.Context, d time.Duration) bool {
		waits = append(waits, d)
		return ctx.Err() == nil
	}
	return b, &waits
}

func TestBellPatterns(t *testing.T) {
	cases := []struct {
		typ   format.SignalType
		rings int
		gap   time.Duration
	}{
		{format.SignalSingle, 1, 0},
		{format.SignalDouble, 2, DoubleBellGap},
		{format.SignalContinuous, 17, ContinuousBellGap},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		b, waits := instantBell(&buf)
		if err := b.play(context.Background(), c.typ); err != nil {
			t.Fatal(err)
		}
		if got := bytes.Count(buf.Bytes(), []byte{'\a'}); got != c.rings {
			t.Errorf("%s: %d rings, want %d", c.typ, got, c.rings)
		}
		for _, w := range *waits {
			if w != c.gap {
				t.Errorf("%s: gap %v, want %v", c.typ, w, c.gap)
			}
		}
	}
}

func TestContinuousCanBeCancelled(t *testing.T) {
	bell := &safeBuffer{}
	p := newPlayer(&bellBackend{w: bell}, discard)
	p.Play(format.SignalContinuous)
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	p.Stop()
	p.Close()
	if time.Since(start) > 400*time.Millisecond {
		t.Fatal("stop must end the continuous beep quickly")
	}
	rings := bytes.Count([]byte(bell.String()), []byte{'\a'})
	time.Sleep(700 * time.Millisecond)
	if after := bytes.Count([]byte(bell.String()), []byte{'\a'}); after != rings || rings > 2 {
		t.Fatalf("bell kept ringing after stop: %d → %d", rings, after)
	}
}

func TestStopKillsPlayerProcess(t *testing.T) {
	started := make(chan struct{})
	finished := make(chan error, 1)
	run := func(ctx context.Context, _ string, _ ...string) error {
		close(started)
		<-ctx.Done() // like exec.CommandContext killing the process
		finished <- ctx.Err()
		return ctx.Err()
	}
	b := &commandBackend{bin: "pw-play", files: map[format.SignalType]string{format.SignalContinuous: "c.wav"}, run: run}
	p := newPlayer(b, discard)
	p.Play(format.SignalContinuous)
	<-started
	p.Stop()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("player process was not stopped")
	}
	p.Close()
}

func TestPlayReplacesPrevious(t *testing.T) {
	var mu sync.Mutex
	var cancelled int
	run := func(ctx context.Context, _ string, args ...string) error {
		if filepath.Base(args[len(args)-1]) == "continuous_beep.wav" {
			<-ctx.Done()
			mu.Lock()
			cancelled++
			mu.Unlock()
		}
		return nil
	}
	b := &commandBackend{bin: "paplay", files: map[format.SignalType]string{
		format.SignalContinuous: "continuous_beep.wav",
		format.SignalSingle:     "single_beep.wav",
	}, run: run}
	p := newPlayer(b, discard)
	p.Play(format.SignalContinuous)
	time.Sleep(20 * time.Millisecond)
	p.Play(format.SignalSingle)
	p.Close()
	mu.Lock()
	defer mu.Unlock()
	if cancelled != 1 {
		t.Fatal("a new signal must cancel the previous one")
	}
}

func TestParseMode(t *testing.T) {
	for _, s := range []string{"auto", "bell", "off"} {
		if m, ok := ParseMode(s); !ok || string(m) != s {
			t.Errorf("%s rejected", s)
		}
	}
	if _, ok := ParseMode("loud"); ok {
		t.Error("unknown mode accepted")
	}
}
