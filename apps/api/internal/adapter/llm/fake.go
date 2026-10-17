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
			{"text": "Read the GDD and notes", "done": false},
			{"text": "Write the design for: " + truncate(request, 60), "done": false},
			{"text": "Record decisions", "done": false},
		}})
		r.Text = "Fake agent here (no LLM configured). I will sketch the design so the pipeline can be tested."
		return r, nil
	case 1:
		return call("read_file", map[string]string{"path": "docs/GDD.md"}), nil
	case 2:
		gdd := fmt.Sprintf("# Game Design Document\n\n## Idea\n%s\n\n## Latest request\n%s\n\n## Core loop\nTap to act, avoid obstacles, score goes up.\n\n## Controls (touch first)\nSingle tap anywhere.\n\n## Screens\nMenu, play, game over.\n\n## Win / lose\nLose on collision; chase the high score.\n\n## Art direction\nPixel art, 32x32 sprites, limited palette.\n", idea, request)
		return call("write_file", map[string]string{"path": "docs/GDD.md", "content": gdd}), nil
	case 3:
		return call("note", map[string]string{"text": "Fake agent run: GDD drafted, no game code changed yet."}), nil
	default:
		return call("finish", map[string]string{"summary": "Drafted docs/GDD.md from the idea (fake agent; set FORGE_LLM=anthropic for real work)."}), nil
	}
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
