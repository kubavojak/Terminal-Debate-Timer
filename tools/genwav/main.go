// Command genwav generates the bundled beep sounds in assets/sounds.
//
//	go generate ./assets
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"log"
	"math"
	"os"
	"path/filepath"
)

const (
	sampleRate = 44100
	freq       = 880.0
	amplitude  = 0.55
	fade       = 0.006 // seconds of fade in/out against clicks
)

type segment struct {
	on  bool
	sec float64
}

func main() {
	out := flag.String("out", "sounds", "output directory")
	flag.Parse()

	// 5 s of 0.2 s beeps every 0.3 s, the same rhythm as the BEL fallback.
	var continuous []segment
	for i := 0; i < 16; i++ {
		continuous = append(continuous, segment{true, 0.2}, segment{false, 0.1})
	}
	continuous = append(continuous, segment{true, 0.2})

	files := map[string][]segment{
		"single_beep.wav":     {{true, 0.35}},
		"double_beep.wav":     {{true, 0.3}, {false, 0.15}, {true, 0.3}},
		"continuous_beep.wav": continuous,
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	for name, segs := range files {
		if err := os.WriteFile(filepath.Join(*out, name), wav(render(segs)), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

func render(segs []segment) []int16 {
	var out []int16
	for _, s := range segs {
		n := int(s.sec * sampleRate)
		for i := 0; i < n; i++ {
			if !s.on {
				out = append(out, 0)
				continue
			}
			t := float64(i) / sampleRate
			env := 1.0
			if t < fade {
				env = t / fade
			} else if rest := s.sec - t; rest < fade {
				env = rest / fade
			}
			v := math.Sin(2*math.Pi*freq*t) + 0.25*math.Sin(2*math.Pi*2*freq*t)
			out = append(out, int16(v/1.25*amplitude*env*math.MaxInt16))
		}
	}
	return out
}

func wav(samples []int16) []byte {
	var b bytes.Buffer
	dataLen := uint32(len(samples) * 2)
	w := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	w(36 + dataLen)
	b.WriteString("WAVEfmt ")
	w(uint32(16))         // fmt chunk size
	w(uint16(1))          // PCM
	w(uint16(1))          // mono
	w(uint32(sampleRate)) // sample rate
	w(uint32(sampleRate * 2))
	w(uint16(2))  // block align
	w(uint16(16)) // bits per sample
	b.WriteString("data")
	w(dataLen)
	w(samples)
	return b.Bytes()
}
