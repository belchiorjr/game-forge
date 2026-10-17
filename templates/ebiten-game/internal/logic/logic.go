// Package logic holds the pure game rules. It must not import Ebitengine: everything here is
// deterministic (randomness and input are passed in), so it can be unit-tested without a GPU.
package logic

const (
	Width  = 360 // logical resolution, portrait
	Height = 640

	GroundY      = 520.0
	PlayerX      = 72.0
	PlayerSize   = 32.0
	gravity      = 0.55
	jumpVelocity = -11.0
	maxObstacles = 8
	baseSpeed    = 4.0
)

type Scene int

const (
	SceneMenu Scene = iota
	ScenePlay
	SceneGameOver
)

func (s Scene) String() string {
	return [...]string{"menu", "play", "gameover"}[s]
}

// RNG is the randomness the rules need; tests pass a fixed sequence.
type RNG interface{ IntN(n int) int }

type Obstacle struct {
	X, W, H float64
	Active  bool
}

// World is the whole game state. Obstacles live in a fixed array so Step never allocates.
type World struct {
	Scene     Scene
	Tick      int
	PlayerY   float64
	VelocityY float64
	Score     int
	Best      int
	Obstacles [maxObstacles]Obstacle
	nextSpawn int
	rng       RNG
}

func NewWorld(rng RNG) *World {
	w := &World{rng: rng}
	w.reset()
	w.Scene = SceneMenu
	return w
}

func (w *World) reset() {
	w.PlayerY, w.VelocityY, w.Score, w.Tick = GroundY-PlayerSize, 0, 0, 0
	w.Obstacles = [maxObstacles]Obstacle{}
	w.nextSpawn = 60
}

func (w *World) OnGround() bool { return w.PlayerY >= GroundY-PlayerSize }

func (w *World) Speed() float64 { return baseSpeed + float64(w.Score)/40 }

// Step advances one tick (1/60 s). tap is true on the tick the player pressed.
func (w *World) Step(tap bool) {
	switch w.Scene {
	case SceneMenu:
		if tap {
			w.reset()
			w.Scene = ScenePlay
		}
		return
	case SceneGameOver:
		if tap {
			w.Scene = SceneMenu
		}
		return
	}

	w.Tick++
	if tap && w.OnGround() {
		w.VelocityY = jumpVelocity
	}
	w.VelocityY += gravity
	w.PlayerY += w.VelocityY
	if w.OnGround() {
		w.PlayerY, w.VelocityY = GroundY-PlayerSize, 0
	}

	speed := w.Speed()
	for i := range w.Obstacles {
		o := &w.Obstacles[i]
		if !o.Active {
			continue
		}
		o.X -= speed
		if o.X+o.W < 0 {
			o.Active = false
			w.Score++
			continue
		}
		if overlaps(PlayerX, w.PlayerY, PlayerSize, PlayerSize, o.X, GroundY-o.H, o.W, o.H) {
			w.Scene = SceneGameOver
			if w.Score > w.Best {
				w.Best = w.Score
			}
			return
		}
	}

	w.nextSpawn--
	if w.nextSpawn <= 0 {
		w.spawn()
		w.nextSpawn = 55 + w.rng.IntN(50)
	}
}

func (w *World) spawn() {
	for i := range w.Obstacles {
		if !w.Obstacles[i].Active {
			w.Obstacles[i] = Obstacle{X: Width, W: 20 + float64(w.rng.IntN(16)), H: 28 + float64(w.rng.IntN(28)), Active: true}
			return
		}
	}
}

func overlaps(ax, ay, aw, ah, bx, by, bw, bh float64) bool {
	return ax < bx+bw && ax+aw > bx && ay < by+bh && ay+ah > by
}
