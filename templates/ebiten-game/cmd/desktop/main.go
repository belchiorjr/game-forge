package main

import (
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"game/internal/game"
	"game/internal/logic"
)

func main() {
	ebiten.SetWindowSize(logic.Width, logic.Height) // phone-sized window for fast local runs
	ebiten.SetWindowTitle("Game")
	if err := ebiten.RunGame(game.New(uint64(time.Now().UnixNano()))); err != nil {
		log.Fatal(err)
	}
}
