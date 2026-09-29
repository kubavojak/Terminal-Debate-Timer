package i18n

import (
	"strings"
	"testing"

	"debtime/internal/format"
)

func TestCatalogsHaveSameKeys(t *testing.T) {
	for k := range cs {
		if _, ok := en[k]; !ok {
			t.Errorf("en is missing %q", k)
		}
	}
	for k := range en {
		if _, ok := cs[k]; !ok {
			t.Errorf("cs is missing %q", k)
		}
	}
}

func TestPlaceholdersMatch(t *testing.T) {
	for k, v := range cs {
		if strings.Count(v, "%") != strings.Count(en[k], "%") {
			t.Errorf("%q: placeholders differ: %q vs %q", k, v, en[k])
		}
	}
}

func TestAllFormatKeysTranslated(t *testing.T) {
	for _, lang := range Supported() {
		tr := New(lang)
		for _, f := range format.All(format.Options{Consultation: true}) {
			if tr.FormatName(f) == f.NameKey {
				t.Errorf("%s: format %q not translated", lang, f.NameKey)
			}
			for _, s := range f.Steps {
				if _, ok := catalogs[lang][s.NameKey]; !ok {
					t.Errorf("%s: step key %q missing", lang, s.NameKey)
				}
			}
			for _, p := range f.PrepPools {
				if _, ok := catalogs[lang][p.NameKey]; !ok {
					t.Errorf("%s: pool key %q missing", lang, p.NameKey)
				}
			}
		}
	}
}

func TestFallbacks(t *testing.T) {
	tr := New("de")
	if tr.Lang() != "cs" {
		t.Fatalf("unknown language must fall back to cs, got %s", tr.Lang())
	}
	if got := tr.T("noSuchKey"); got != "noSuchKey" {
		t.Fatalf("missing key must return the key, got %q", got)
	}
	cs["onlyInCsTest"] = "jen česky"
	defer delete(cs, "onlyInCsTest")
	if got := New("en").T("onlyInCsTest"); got != "jen česky" {
		t.Fatalf("missing en key must fall back to cs, got %q", got)
	}
}

func TestStepNames(t *testing.T) {
	trEn := New("en")
	bp := format.BritishParliamentary()
	if got := trEn.StepName(bp.Steps[2]); got != "Leader of the Opposition · OO 1" {
		t.Errorf("got %q", got)
	}
	if got := trEn.StepName(bp.Steps[0]); got != "Preparation" {
		t.Errorf("got %q", got)
	}
	trCs := New("cs")
	if got := trCs.StepName(format.Resitelska(false).Steps[1]); got != "Dotazování O × H" {
		t.Errorf("got %q", got)
	}
	if got := trCs.StepName(format.KarlPopper(nil).Steps[4]); got != "Křížový výslech N3 → A1" {
		t.Errorf("got %q", got)
	}
}

func TestDetect(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{}, "cs"},
		{map[string]string{"LANG": "en_US.UTF-8"}, "en"},
		{map[string]string{"LANG": "cs_CZ.UTF-8"}, "cs"},
		{map[string]string{"LANG": "de_DE.UTF-8"}, "cs"},
		{map[string]string{"LANG": "cs_CZ.UTF-8", "LC_ALL": "en_GB.UTF-8"}, "en"},
	}
	for _, c := range cases {
		if got := Detect(func(k string) string { return c.env[k] }); got != c.want {
			t.Errorf("%v: got %s, want %s", c.env, got, c.want)
		}
	}
}
