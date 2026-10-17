//go:build !(js && wasm)

package debug

// Install is a no-op outside the browser.
func Install(Target) {}
