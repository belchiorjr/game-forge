// Package debug exposes window.__game in the WASM build (see debug_js.go).
package debug

type Target interface {
	Tap()
	Advance(n int)
	Frame() int
	StateJSON() string
}
