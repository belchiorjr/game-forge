//go:build js && wasm

package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"game/internal/debug"
	"game/internal/game"
)

func main() {
	g := game.New(1) // fixed seed: previews and smoke tests are reproducible
	debug.Install(g)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
