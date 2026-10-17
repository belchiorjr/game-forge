package logic

import "testing"

type fixedRNG struct{ v int }

func (r fixedRNG) IntN(n int) int { return r.v % n }

func TestTapStartsGame(t *testing.T) {
	w := NewWorld(fixedRNG{})
	if w.Scene != SceneMenu {
		t.Fatalf("scene = %v, want menu", w.Scene)
	}
	w.Step(true)
	if w.Scene != ScenePlay {
		t.Fatalf("scene = %v, want play", w.Scene)
	}
}

func TestJumpOnlyFromGround(t *testing.T) {
	w := NewWorld(fixedRNG{})
	w.Step(true) // start
	w.Step(true) // jump
	if w.OnGround() {
		t.Fatal("player should be airborne after a jump")
	}
	vy := w.VelocityY
	w.Step(true) // tap mid-air must not jump again
	if w.VelocityY < vy {
		t.Fatal("double jump should not be possible")
	}
	for i := 0; i < 120 && !w.OnGround(); i++ {
		w.Step(false)
	}
	if !w.OnGround() {
		t.Fatal("player should land")
	}
}

func TestCollisionEndsGameAndKeepsBest(t *testing.T) {
	w := NewWorld(fixedRNG{})
	w.Step(true)
	w.Score = 3
	w.Obstacles[0] = Obstacle{X: PlayerX, W: 20, H: 40, Active: true}
	w.Step(false)
	if w.Scene != SceneGameOver || w.Best != 3 {
		t.Fatalf("scene = %v best = %d, want gameover/3", w.Scene, w.Best)
	}
	w.Step(true)
	if w.Scene != SceneMenu {
		t.Fatalf("scene = %v, want menu", w.Scene)
	}
}

func TestObstaclesScoreWhenPassed(t *testing.T) {
	w := NewWorld(fixedRNG{})
	w.Step(true)
	w.Obstacles[0] = Obstacle{X: -19, W: 20, H: 30, Active: true}
	w.Step(false)
	if w.Score != 1 || w.Obstacles[0].Active {
		t.Fatalf("score = %d active = %v", w.Score, w.Obstacles[0].Active)
	}
}

func TestStepDoesNotAllocate(t *testing.T) {
	w := NewWorld(fixedRNG{v: 7})
	w.Step(true)
	allocs := testing.AllocsPerRun(1000, func() {
		w.Step(w.Tick%40 == 0)
		if w.Scene != ScenePlay {
			w.Step(true)
			w.Step(true)
		}
	})
	if allocs != 0 {
		t.Fatalf("Step allocates %.1f times per run; the game loop must not allocate", allocs)
	}
}
