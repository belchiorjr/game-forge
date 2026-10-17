// Package llm holds LLM adapters. Fake replays a scripted session so the whole pipe
// (runs, tools, events, UI) works before any provider is wired or paid for.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type Fake struct {
	Delay time.Duration // pause per step so the UI timeline is watchable
}

func NewFake(delay time.Duration) *Fake { return &Fake{Delay: delay} }

// Complete decides the next step from how many assistant turns already happened.
func (f *Fake) Complete(ctx context.Context, _ string, msgs []port.Message, _ []port.ToolDef) (port.LLMResponse, error) {
	select {
	case <-time.After(f.Delay):
	case <-ctx.Done():
		return port.LLMResponse{}, ctx.Err()
	}
	turn := 0
	for _, m := range msgs {
		if m.Role == "assistant" {
			turn++
		}
	}
	idea, request := field(msgs[0].Text, "Idea: "), section(msgs[0].Text, "--- request ---")
	usage := port.Usage{InputTokens: 900 + 150*turn, OutputTokens: 120}
	call := func(name string, input any) port.LLMResponse {
		raw, _ := json.Marshal(input)
		return port.LLMResponse{ToolCalls: []port.ToolCall{{ID: fmt.Sprintf("fake_%d", turn), Name: name, Input: raw}}, Usage: usage}
	}

	switch turn {
	case 0:
		r := call("update_plan", map[string]any{"items": []map[string]any{
			{"text": "Read the GDD", "done": false},
			{"text": "Design for: " + truncate(request, 50), "done": false},
			{"text": "Restyle the palette", "done": false},
			{"text": "Build, test and preview", "done": false},
		}})
		r.Text = "Fake agent here (no LLM configured). I will restyle the game and run the full verify gate so you can watch the pipeline."
		return r, nil
	case 1:
		return call("read_file", map[string]string{"path": "docs/GDD.md"}), nil
	case 2:
		gdd := fmt.Sprintf("# Game Design Document\n\n## Idea\n%s\n\n## Latest request\n%s\n\n## Core loop\nTap to jump, avoid obstacles, score goes up.\n\n## Controls (touch first)\nSingle tap anywhere.\n\n## Screens\nMenu, play, game over.\n\n## Win / lose\nLose on collision; chase the high score.\n\n## Art direction\nFlat shapes for now; palette in internal/render/palette.go.\n", idea, request)
		return call("write_file", map[string]string{"path": "docs/GDD.md", "content": gdd}), nil
	case 3:
		return call("write_file", map[string]string{"path": "internal/render/palette.go", "content": palette(request)}), nil
	case 4:
		return call("go_fmt", map[string]string{}), nil
	case 5:
		return call("go_vet", map[string]string{}), nil
	case 6:
		return call("go_build", map[string]string{}), nil
	case 7:
		return call("go_test", map[string]string{}), nil
	case 8:
		r := call("build_wasm", map[string]string{})
		r.Text = "Tests pass. Building the preview now."
		return r, nil
	case 9:
		return call("note", map[string]string{"text": "Fake agent run: palette restyled from the request text; gate passed."}), nil
	default:
		return call("finish", map[string]string{"summary": "Restyled the palette and passed fmt, vet, build, tests and the WASM build (fake agent; configure Claude in Settings for real work)."}), nil
	}
}

// palette derives a colour scheme from the request text, so each fake run looks different.
func palette(seed string) string {
	h := uint32(2166136261)
	for i := 0; i < len(seed); i++ {
		h = (h ^ uint32(seed[i])) * 16777619
	}
	hue := float64(h%360) / 360
	rgb := func(hh, s, l float64) string {
		r, g, b := hsl(hh, s, l)
		return fmt.Sprintf("color.RGBA{0x%02x, 0x%02x, 0x%02x, 0xff}", r, g, b)
	}
	return fmt.Sprintf(`package render

import "image/color"

// Palette of the game. Change these to restyle everything at once.
var (
	sky      = %s
	ground   = %s
	hero     = %s
	obstacle = %s
)
`, rgb(hue, 0.45, 0.16), rgb(hue, 0.30, 0.28), rgb(hue+0.5, 0.95, 0.60), rgb(hue, 0.15, 0.92))
}

func hsl(h, s, l float64) (uint8, uint8, uint8) {
	h -= float64(int(h))
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	conv := func(t float64) uint8 {
		switch {
		case t < 0:
			t++
		case t > 1:
			t--
		}
		var v float64
		switch {
		case t < 1.0/6:
			v = p + (q-p)*6*t
		case t < 0.5:
			v = q
		case t < 2.0/3:
			v = p + (q-p)*(2.0/3-t)*6
		default:
			v = p
		}
		return uint8(v*255 + 0.5)
	}
	return conv(h + 1.0/3), conv(h), conv(h - 1.0/3)
}

func field(text, prefix string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

func section(text, header string) string {
	_, after, _ := strings.Cut(text, header)
	return strings.TrimSpace(after)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
