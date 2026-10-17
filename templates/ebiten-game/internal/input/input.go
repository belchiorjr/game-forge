// Package input unifies touch, mouse and keyboard so gameplay never reads Ebitengine directly.
package input

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Pointer struct {
	touchIDs []ebiten.TouchID // reused every frame to avoid allocations
	injected bool             // set by tests / the debug hook
}

// Inject queues a tap for the next frame (used by the WASM debug hook and smoke tests).
func (p *Pointer) Inject() { p.injected = true }

// JustPressed reports a new tap from any source this frame.
func (p *Pointer) JustPressed() bool {
	if p.injected {
		p.injected = false
		return true
	}
	p.touchIDs = inpututil.AppendJustPressedTouchIDs(p.touchIDs[:0])
	return len(p.touchIDs) > 0 ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyArrowUp)
}
