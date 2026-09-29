// Package clock is the time source of the timer. It measures with the
// monotonic clock but falls back to the wall clock when the machine was
// suspended (the monotonic clock stops during suspend on Linux).
package clock

import (
	"sync"
	"time"
)

// SleepThreshold: when the wall clock moved this much more than the monotonic
// clock, the machine was asleep and the wall clock is used.
const SleepThreshold = 2 * time.Second

// Reading is a point in time. Mono is only comparable between readings of the
// same clock in the same process (HasMono). Readings restored from disk have
// only the wall-clock part.
type Reading struct {
	Wall    time.Time
	Mono    time.Duration
	HasMono bool
}

// FromWall makes a wall-clock-only reading, e.g. from saved state.
func FromWall(t time.Time) Reading { return Reading{Wall: t.Round(0)} }

// Clock returns the current time.
type Clock interface {
	Now() Reading
}

// Between returns the time elapsed from start to now, never negative.
func Between(start, now Reading) time.Duration {
	wall := now.Wall.Sub(start.Wall)
	if !start.HasMono || !now.HasMono {
		if wall < 0 {
			return 0
		}
		return wall
	}
	mono := now.Mono - start.Mono
	if mono < 0 {
		mono = 0
	}
	if wall-mono > SleepThreshold {
		return wall
	}
	return mono
}

// Real is the system clock.
type Real struct {
	base time.Time
}

// NewReal returns the system clock.
func NewReal() *Real { return &Real{base: time.Now()} }

// Now implements Clock.
func (r *Real) Now() Reading {
	now := time.Now()
	// now.Sub(base) uses the monotonic readings of both values.
	return Reading{Wall: now.Round(0), Mono: now.Sub(r.base), HasMono: true}
}

// Fake is a manually driven clock for tests.
type Fake struct {
	mu sync.Mutex
	r  Reading
}

// NewFake returns a fake clock set to the given wall time.
func NewFake(wall time.Time) *Fake {
	return &Fake{r: Reading{Wall: wall.Round(0), HasMono: true}}
}

// Now implements Clock.
func (f *Fake) Now() Reading {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.r
}

// Advance moves both clocks forward.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.r.Wall = f.r.Wall.Add(d)
	f.r.Mono += d
}

// Suspend moves only the wall clock, like a machine that was asleep.
func (f *Fake) Suspend(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.r.Wall = f.r.Wall.Add(d)
}

// ShiftWall moves only the wall clock by d (may be negative), like an NTP
// correction while the machine is awake.
func (f *Fake) ShiftWall(d time.Duration) { f.Suspend(d) }
