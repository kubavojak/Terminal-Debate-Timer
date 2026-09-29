package clock

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)

func TestBetweenUsesMonotonic(t *testing.T) {
	f := NewFake(t0)
	start := f.Now()
	f.Advance(90 * time.Second)
	f.ShiftWall(-time.Hour) // NTP jump backwards must not matter
	if got := Between(start, f.Now()); got != 90*time.Second {
		t.Fatalf("got %v, want 90s", got)
	}
}

func TestBetweenSmallWallDriftIgnored(t *testing.T) {
	f := NewFake(t0)
	start := f.Now()
	f.Advance(10 * time.Second)
	f.ShiftWall(time.Second) // below the threshold
	if got := Between(start, f.Now()); got != 10*time.Second {
		t.Fatalf("got %v, want 10s", got)
	}
}

func TestBetweenCountsSuspend(t *testing.T) {
	f := NewFake(t0)
	start := f.Now()
	f.Advance(10 * time.Second)
	f.Suspend(5 * time.Minute)
	if got := Between(start, f.Now()); got != 5*time.Minute+10*time.Second {
		t.Fatalf("got %v", got)
	}
}

func TestBetweenWallOnly(t *testing.T) {
	f := NewFake(t0)
	start := FromWall(t0.Add(-30 * time.Second))
	if got := Between(start, f.Now()); got != 30*time.Second {
		t.Fatalf("got %v", got)
	}
	future := FromWall(t0.Add(time.Minute))
	if got := Between(future, f.Now()); got != 0 {
		t.Fatalf("negative elapsed must clamp to 0, got %v", got)
	}
}

func TestRealIsMonotonic(t *testing.T) {
	c := NewReal()
	a := c.Now()
	b := c.Now()
	if !a.HasMono || Between(a, b) < 0 {
		t.Fatal("real clock readings must be monotonic")
	}
	if a.Wall != a.Wall.Round(0) {
		t.Fatal("wall part must not carry a monotonic reading")
	}
}
