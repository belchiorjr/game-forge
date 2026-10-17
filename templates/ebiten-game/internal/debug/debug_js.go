//go:build js && wasm

package debug

import "syscall/js"

// Install publishes the debug surface the smoke test and the agent's smoke_test tool use.
func Install(t Target) {
	api := js.Global().Get("Object").New()
	api.Set("tap", js.FuncOf(func(js.Value, []js.Value) any { t.Tap(); return nil }))
	api.Set("advance", js.FuncOf(func(_ js.Value, args []js.Value) any {
		n := 1
		if len(args) > 0 {
			n = args[0].Int()
		}
		t.Advance(n)
		return nil
	}))
	api.Set("frame", js.FuncOf(func(js.Value, []js.Value) any { return t.Frame() }))
	api.Set("state", js.FuncOf(func(js.Value, []js.Value) any {
		return js.Global().Get("JSON").Call("parse", t.StateJSON())
	}))
	api.Set("ready", true)
	js.Global().Set("__game", api)
}
