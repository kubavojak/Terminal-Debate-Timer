package main

import (
	"bytes"
	"strings"
	"testing"
)

func runArgs(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("LANG", "cs_CZ.UTF-8")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestPlan(t *testing.T) {
	code, out, _ := runArgs(t, "plan", "bp")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"British Parliamentary", "Příprava", "15:00", "Vůdce opozice · OO 1", "POI 1:00–6:00", "7:15 •••", "Celkem: 1:11:00"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestPlanKarlPopper(t *testing.T) {
	_, out, _ := runArgs(t, "plan", "kp")
	for _, want := range []string{"Křížový výslech N3 → A1", "Příprava A", "Přípravné fondy", "Celkem: 1:48:00"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestPlanWithFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--lang", "en", "plan", "custom", "--speakers", "2", "--minutes", "3"},
		{"plan", "custom", "--lang", "en", "--speakers", "2", "--minutes", "3"},
	} {
		code, out, errOut := runArgs(t, args...)
		if code != 0 {
			t.Fatalf("%v: exit %d %s", args, code, errOut)
		}
		for _, want := range []string{"Custom format", "Speaker 2", "Total: 21:00"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: missing %q in:\n%s", args, want, out)
			}
		}
		if strings.Contains(out, "Speaker 3") {
			t.Errorf("%v: --speakers ignored", args)
		}
	}
	if code, _, _ := runArgs(t, "plan", "bp", "extra"); code != 2 {
		t.Errorf("extra argument: exit %d", code)
	}
	if code, _, _ := runArgs(t, "start"); code != 2 {
		t.Errorf("unknown command: exit %d", code)
	}
}

func TestPlanErrors(t *testing.T) {
	if code, _, _ := runArgs(t, "plan"); code != 2 {
		t.Errorf("missing format: exit %d", code)
	}
	if code, _, errOut := runArgs(t, "plan", "nope"); code != 2 || !strings.Contains(errOut, "nope") {
		t.Errorf("unknown format: exit %d, %q", code, errOut)
	}
}

func TestListFormatsAndVersion(t *testing.T) {
	code, out, _ := runArgs(t, "--list-formats", "--lang", "en")
	if code != 0 || !strings.Contains(out, "custom") || !strings.Contains(out, "Custom format") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if lines := strings.Count(out, "\n"); lines != 6 {
		t.Errorf("%d formats listed", lines)
	}
	code, out, _ = runArgs(t, "--version")
	if code != 0 || !strings.HasPrefix(out, "debtime ") {
		t.Fatalf("version: %d %q", code, out)
	}
}

func TestBadFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--format", "xx"},
		{"--sound", "loud"},
		{"--lang", "de"},
		{"--nope"},
	} {
		if code, _, _ := runArgs(t, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
