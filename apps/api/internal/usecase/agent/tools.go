package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

// toolEnv is what a tool handler may touch during one run.
type toolEnv struct {
	ctx       context.Context
	projectID string
	runID     string
	ws        port.Workspace
	sandbox   port.Sandbox
	bus       port.EventBus
	passed    map[string]bool // gate checks that passed since the last code change
	maxFile   int
	finished  bool
	summary   string
}

type tool struct {
	def     port.ToolDef
	handler func(env *toolEnv, input json.RawMessage) (string, error)
}

func schema(props string, required ...string) []byte {
	req, _ := json.Marshal(required)
	return []byte(fmt.Sprintf(`{"type":"object","properties":{%s},"required":%s}`, props, req))
}

func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("invalid input: %w", err)
	}
	return v, nil
}

func (e *toolEnv) publish(t domain.EventType, data any) {
	_ = e.bus.Publish(context.WithoutCancel(e.ctx), e.projectID, e.runID, t, data)
}

func registry() map[string]tool {
	tools := []tool{
		{
			def: port.ToolDef{Name: "list_files", Description: "List files under a directory of the project (recursive, skips .git).",
				Schema: schema(`"dir":{"type":"string","description":"relative directory, empty for root"}`)},
			handler: func(env *toolEnv, raw json.RawMessage) (string, error) {
				in, err := decode[struct{ Dir string }](raw)
				if err != nil {
					return "", err
				}
				files, err := env.ws.List(env.ctx, env.projectID, in.Dir)
				if err != nil {
					return "", err
				}
				var b strings.Builder
				for _, f := range files {
					if f.IsDir {
						fmt.Fprintf(&b, "%s/\n", f.Path)
					} else {
						fmt.Fprintf(&b, "%s (%d bytes)\n", f.Path, f.Size)
					}
				}
				return b.String(), nil
			},
		},
		{
			def: port.ToolDef{Name: "read_file", Description: "Read a text file of the project.",
				Schema: schema(`"path":{"type":"string"}`, "path")},
			handler: func(env *toolEnv, raw json.RawMessage) (string, error) {
				in, err := decode[struct{ Path string }](raw)
				if err != nil {
					return "", err
				}
				data, err := env.ws.ReadFile(env.ctx, env.projectID, in.Path)
				return string(data), err
			},
		},
		{
			def: port.ToolDef{Name: "write_file", Description: "Create or replace a text file.",
				Schema: schema(`"path":{"type":"string"},"content":{"type":"string"}`, "path", "content")},
			handler: func(env *toolEnv, raw json.RawMessage) (string, error) {
				in, err := decode[struct{ Path, Content string }](raw)
				if err != nil {
					return "", err
				}
				if len(in.Content) > env.maxFile {
					return "", fmt.Errorf("%w: content larger than %d bytes", domain.ErrLimitExceeded, env.maxFile)
				}
				if err := env.ws.WriteFile(env.ctx, env.projectID, in.Path, []byte(in.Content)); err != nil {
					return "", err
				}
				env.codeChanged(in.Path)
				env.publish(domain.EventFileChanged, map[string]string{"path": in.Path, "kind": "modified"})
				return "ok", nil
			},
		},
		{
			def: port.ToolDef{Name: "edit_file", Description: "Replace one exact, unique occurrence of old with new in a file.",
				Schema: schema(`"path":{"type":"string"},"old":{"type":"string"},"new":{"type":"string"}`, "path", "old", "new")},
			handler: func(env *toolEnv, raw json.RawMessage) (string, error) {
				in, err := decode[struct{ Path, Old, New string }](raw)
				if err != nil {
					return "", err
				}
				data, err := env.ws.ReadFile(env.ctx, env.projectID, in.Path)
				if err != nil {
					return "", err
				}
				if n := strings.Count(string(data), in.Old); n != 1 || in.Old == "" {
					return "", fmt.Errorf("old text must occur exactly once, found %d", n)
				}
				out := strings.Replace(string(data), in.Old, in.New, 1)
				if len(out) > env.maxFile {
					return "", fmt.Errorf("%w: file larger than %d bytes", domain.ErrLimitExceeded, env.maxFile)
				}
				if err := env.ws.WriteFile(env.ctx, env.projectID, in.Path, []byte(out)); err != nil {
					return "", err
				}
				env.codeChanged(in.Path)
				env.publish(domain.EventFileChanged, map[string]string{"path": in.Path, "kind": "modified"})
				return "ok", nil
			},
		},
		{
			def: port.ToolDef{Name: "update_plan", Description: "Replace the visible todo list.",
				Schema: schema(`"items":{"type":"array","items":{"type":"object","properties":{"text":{"type":"string"},"done":{"type":"boolean"}},"required":["text"]}}`, "items")},
			handler: func(env *toolEnv, raw json.RawMessage) (string, error) {
				in, err := decode[struct {
					Items []struct {
						Text string `json:"text"`
						Done bool   `json:"done"`
					} `json:"items"`
				}](raw)
				if err != nil {
					return "", err
				}
				env.publish(domain.EventAgentPlan, map[string]any{"items": in.Items})
				return "ok", nil
			},
		},
		{
			def: port.ToolDef{Name: "note", Description: "Append a line to docs/AGENT_NOTES.md (memory across runs).",
				Schema: schema(`"text":{"type":"string"}`, "text")},
			handler: func(env *toolEnv, raw json.RawMessage) (string, error) {
				in, err := decode[struct{ Text string }](raw)
				if err != nil {
					return "", err
				}
				const path = "docs/AGENT_NOTES.md"
				old, _ := env.ws.ReadFile(env.ctx, env.projectID, path)
				content := strings.TrimRight(string(old), "\n") + "\n- " + strings.TrimSpace(in.Text) + "\n"
				if err := env.ws.WriteFile(env.ctx, env.projectID, path, []byte(content)); err != nil {
					return "", err
				}
				env.publish(domain.EventFileChanged, map[string]string{"path": path, "kind": "modified"})
				return "ok", nil
			},
		},
		{
			def: port.ToolDef{Name: "finish", Description: "Declare the requested work done, with a short summary for the person. Rejected unless go_fmt, go_vet, go_build, go_test and build_wasm all passed after your last code change.",
				Schema: schema(`"summary":{"type":"string"}`, "summary")},
			handler: func(env *toolEnv, raw json.RawMessage) (string, error) {
				in, err := decode[struct{ Summary string }](raw)
				if err != nil {
					return "", err
				}
				if report := env.gateReport(); report != "" {
					return "", errors.New(report)
				}
				env.finished, env.summary = true, in.Summary
				return "ok", nil
			},
		},
	}
	tools = append(tools, checkTools()...)
	m := make(map[string]tool, len(tools))
	for _, t := range tools {
		m[t.def.Name] = t
	}
	return m
}
