// Package assets bundles the sound files into the binary.
package assets

import "embed"

//go:generate go run ../tools/genwav -out sounds

// Sounds holds sounds/single_beep.wav, double_beep.wav and continuous_beep.wav.
//
//go:embed sounds/*.wav
var Sounds embed.FS
