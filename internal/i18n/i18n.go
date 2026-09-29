// Package i18n holds the Czech and English texts. Keys follow the Flutter
// app (app_cs.arb / app_en.arb) where one exists.
package i18n

import (
	"fmt"
	"strings"

	"debtime/internal/format"
)

// Default is the fallback language.
const Default = "cs"

var catalogs = map[string]map[string]string{
	"cs": cs,
	"en": en,
}

// Supported lists the available languages.
func Supported() []string { return []string{"cs", "en"} }

// Detect picks the language from LC_ALL, LC_MESSAGES or LANG; cs when the
// variable is empty or names another language.
func Detect(getenv func(string) string) string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.ToLower(getenv(k))
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "en") {
			return "en"
		}
		return Default
	}
	return Default
}

// Translator returns texts in one language.
type Translator struct {
	lang string
}

// New returns a translator; unknown languages use the default.
func New(lang string) Translator {
	if _, ok := catalogs[lang]; !ok {
		lang = Default
	}
	return Translator{lang: lang}
}

// Lang returns the language code.
func (t Translator) Lang() string { return t.lang }

// T translates key. A missing key falls back to Czech, then to the key.
func (t Translator) T(key string) string {
	if s, ok := catalogs[t.lang][key]; ok {
		return s
	}
	if s, ok := catalogs[Default][key]; ok {
		return s
	}
	return key
}

// F translates key and formats it with args.
func (t Translator) F(key string, args ...any) string {
	return fmt.Sprintf(t.T(key), args...)
}

// Label fills a translated name with a label: substituted for %s when the
// text has one, otherwise appended after a middle dot.
func (t Translator) Label(key, label string) string {
	name := t.T(key)
	if strings.Contains(name, "%s") {
		return fmt.Sprintf(name, label)
	}
	if label == "" {
		return name
	}
	return name + " · " + label
}

// StepName is the display name of a step.
func (t Translator) StepName(s format.Step) string { return t.Label(s.NameKey, s.Label) }

// PoolName is the display name of a preparation pool.
func (t Translator) PoolName(p format.PrepPool) string { return t.T(p.NameKey) }

// FormatName is the display name of a format.
func (t Translator) FormatName(f format.Format) string { return t.T(f.NameKey) }
