package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

// Gate checks, in the order the agent should run them. All must pass after the last code edit
// before finish is accepted. The gate is code, not a prompt: the model cannot talk its way past it.
var gateChecks = []string{"go_fmt", "go_vet", "go_build", "go_test", "build_wasm"}

var wasmEnv = map[string]string{"GOOS": "js", "GOARCH": "wasm"}

type checkSpec struct {
	desc    string
	args    func(input json.RawMessage) ([]string, error)
	name    string
	env     map[string]string
	timeout time.Duration
}

func fixed(args ...string) func(json.RawMessage) ([]string, error) {
	return func(json.RawMessage) ([]string, error) { return args, nil }
}

var checkSpecs = map[string]checkSpec{
	"go_fmt": {desc: "Format all Go files in place (gofmt -l -w .). Lists the files it rewrote.",
		name: "gofmt", args: fixed("-l", "-w", "."), timeout: time.Minute},
	"go_vet": {desc: "Run go vet ./... for the js/wasm target.",
		name: "go", args: fixed("vet", "./..."), env: wasmEnv, timeout: 2 * time.Minute},
	"go_build": {desc: "Compile every package (go build ./...) for the js/wasm target. Returns compiler errors.",
		name: "go", args: fixed("build", "./..."), env: wasmEnv, timeout: 2 * time.Minute},
	"go_test": {desc: "Run unit tests (go test ./... natively). Optional pkg narrows it, e.g. ./internal/logic/...",
		name: "go", timeout: 3 * time.Minute,
		args: func(raw json.RawMessage) ([]string, error) {
			in, err := decode[struct{ Pkg string }](raw)
			if err != nil {
				return nil, err
			}
			pkg := strings.TrimSpace(in.Pkg)
			if pkg == "" {
				pkg = "./..."
			}
			if !strings.HasPrefix(pkg, "./") {
				return nil, fmt.Errorf("pkg must start with ./")
			}
			return []string{"test", "-count=1", pkg}, nil
		}},
	"build_wasm": {desc: "Build the WebAssembly preview (web/game.wasm). On success the person sees the new build live in the preview pane.",
		name: "go", args: fixed("build", "-o", "web/game.wasm", "./cmd/wasm"), env: wasmEnv, timeout: 3 * time.Minute},
}

func checkTools() []tool {
	out := make([]tool, 0, len(checkSpecs))
	for _, name := range gateChecks {
		name, spec := name, checkSpecs[name]
		props := ""
		if name == "go_test" {
			props = `"pkg":{"type":"string","description":"package pattern, default ./..."}`
		}
		out = append(out, tool{
			def: port.ToolDef{Name: name, Description: spec.desc, Schema: schema(props)},
			handler: func(env *toolEnv, raw json.RawMessage) (string, error) {
				return env.runCheck(name, spec, raw)
			},
		})
	}
	return out
}

func (e *toolEnv) runCheck(name string, spec checkSpec, raw json.RawMessage) (string, error) {
	args, err := spec.args(raw)
	if err != nil {
		return "", err
	}
	if name == "build_wasm" {
		e.publish(domain.EventBuildStarted, map[string]string{"target": "wasm"})
	}
	start := time.Now()
	res, err := e.sandbox.Run(e.ctx, port.CommandSpec{ProjectID: e.projectID, Name: spec.name, Args: args, Env: spec.env, Timeout: spec.timeout, MaxOutput: 24 << 10})
	ok := err == nil && res.ExitCode == 0 && !res.TimedOut
	if name == "build_wasm" {
		e.publish(domain.EventBuildFinished, map[string]any{"target": "wasm", "ok": ok, "duration_ms": time.Since(start).Milliseconds(), "output": truncate(res.Output, 2000)})
	}
	if err != nil {
		e.failCheck(name)
		return "", err
	}
	if name == "go_fmt" {
		for _, f := range strings.Fields(res.Output) {
			e.publish(domain.EventFileChanged, map[string]string{"path": f, "kind": "modified"})
		}
	}
	// A narrowed go_test run does not count for the gate: the whole suite must pass.
	partial := name == "go_test" && len(args) > 2 && args[2] != "./..."
	switch {
	case res.TimedOut:
		e.failCheck(name)
		return "", fmt.Errorf("timed out after %s\n%s", spec.timeout, res.Output)
	case res.ExitCode != 0:
		e.failCheck(name)
		return "", fmt.Errorf("exit %d\n%s", res.ExitCode, res.Output)
	case !partial:
		e.passed[name] = true
	}
	if strings.TrimSpace(res.Output) == "" {
		return "ok", nil
	}
	return res.Output, nil
}

func (e *toolEnv) failCheck(name string) { delete(e.passed, name) }

// codeChanged invalidates the gate. Docs changes (GDD, notes) do not affect the build.
func (e *toolEnv) codeChanged(path string) {
	if !strings.HasPrefix(strings.TrimPrefix(path, "./"), "docs/") {
		e.passed = map[string]bool{}
	}
}

// gateReport returns "" when finish may be accepted.
func (e *toolEnv) gateReport() string {
	var missing []string
	for _, c := range gateChecks {
		if !e.passed[c] {
			missing = append(missing, c)
		}
	}
	if len(missing) == 0 {
		return ""
	}
	return "verify gate: not done yet. Since your last code change these checks have not passed: " +
		strings.Join(missing, ", ") + ". Run them (fix any failure), then call finish again."
}
