// Package mobile is the ebitenmobile entry point:
//
//	ebitenmobile bind -target android -javapkg com.example.game -o game.aar ./mobile
//	ebitenmobile bind -target ios -o Game.xcframework ./mobile
package mobile

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2/mobile"

	"game/internal/game"
)

func init() {
	mobile.SetGame(game.New(uint64(time.Now().UnixNano())))
}

// Dummy forces gomobile to export the package.
func Dummy() {}
