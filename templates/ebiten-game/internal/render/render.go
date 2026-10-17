// Package render draws the logic state. It only reads the world.
package render

import (
	"image/color"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"game/internal/logic"
)

var (
	sky      = color.RGBA{0x1b, 0x1e, 0x3c, 0xff}
	ground   = color.RGBA{0x3a, 0x2e, 0x4f, 0xff}
	hero     = color.RGBA{0xff, 0x7a, 0x33, 0xff}
	obstacle = color.RGBA{0xe8, 0xe6, 0xf0, 0xff}
)

// Renderer caches HUD strings so Draw does not format text every frame.
type Renderer struct {
	lastScore, lastBest int
	scoreText, bestText string
}

func New() *Renderer { return &Renderer{lastScore: -1, lastBest: -1} }

func (r *Renderer) Draw(screen *ebiten.Image, w *logic.World) {
	screen.Fill(sky)
	vector.DrawFilledRect(screen, 0, logic.GroundY, logic.Width, logic.Height-logic.GroundY, ground, false)
	for i := range w.Obstacles {
		o := &w.Obstacles[i]
		if o.Active {
			vector.DrawFilledRect(screen, float32(o.X), float32(logic.GroundY-o.H), float32(o.W), float32(o.H), obstacle, false)
		}
	}
	vector.DrawFilledRect(screen, logic.PlayerX, float32(w.PlayerY), logic.PlayerSize, logic.PlayerSize, hero, false)

	if w.Score != r.lastScore {
		r.lastScore, r.scoreText = w.Score, "SCORE "+strconv.Itoa(w.Score)
	}
	if w.Best != r.lastBest {
		r.lastBest, r.bestText = w.Best, "BEST "+strconv.Itoa(w.Best)
	}
	ebitenutil.DebugPrintAt(screen, r.scoreText, 16, 48) // below the typical notch area
	ebitenutil.DebugPrintAt(screen, r.bestText, 16, 64)

	switch w.Scene {
	case logic.SceneMenu:
		ebitenutil.DebugPrintAt(screen, "TAP TO START", logic.Width/2-36, logic.Height/2)
	case logic.SceneGameOver:
		ebitenutil.DebugPrintAt(screen, "GAME OVER - TAP", logic.Width/2-45, logic.Height/2)
	}
}
