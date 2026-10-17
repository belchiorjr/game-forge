// Package game wires logic, input and render into an ebiten.Game.
package game

import (
	"encoding/json"
	"math/rand/v2"

	"github.com/hajimehoshi/ebiten/v2"

	"game/internal/input"
	"game/internal/logic"
	"game/internal/render"
)

type Game struct {
	World    *logic.World
	pointer  input.Pointer
	renderer *render.Renderer
}

func New(seed uint64) *Game {
	return &Game{
		World:    logic.NewWorld(rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))),
		renderer: render.New(),
	}
}

// Update runs at a fixed 60 ticks per second on every device.
func (g *Game) Update() error {
	g.World.Step(g.pointer.JustPressed())
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) { g.renderer.Draw(screen, g.World) }

// Layout keeps a fixed logical resolution; Ebitengine scales and letterboxes it.
func (g *Game) Layout(_, _ int) (int, int) { return logic.Width, logic.Height }

// ---- debug surface used by the WASM hook and smoke tests ----

func (g *Game) Tap() { g.pointer.Inject() }

// Advance runs n ticks immediately, without waiting for frames, so tests are fast and deterministic.
func (g *Game) Advance(n int) {
	for i := 0; i < n; i++ {
		_ = g.Update()
	}
}

func (g *Game) Frame() int { return g.World.Tick }

func (g *Game) StateJSON() string {
	active := 0
	for _, o := range g.World.Obstacles {
		if o.Active {
			active++
		}
	}
	b, _ := json.Marshal(map[string]any{
		"scene": g.World.Scene.String(), "tick": g.World.Tick, "score": g.World.Score,
		"best": g.World.Best, "player_y": g.World.PlayerY, "obstacles": active,
		"game_over": g.World.Scene == logic.SceneGameOver,
	})
	return string(b)
}
