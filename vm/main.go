// Package main is the entry point of the program.
package main

import (
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	initialScreenWidth  = 800
	initialScreenHeight = 600
)

type Game struct {
}

func (g *Game) Update() error {
	// Update game logic here
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{G: 255})
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	// Return the layout of the game
	return outsideWidth, outsideHeight
}

func main() {
	ebiten.SetWindowTitle("RTv1 - Shader - Go")
	ebiten.SetWindowSize(initialScreenWidth, initialScreenHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	g := &Game{}
	if err := ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{
		InitUnfocused: true,
	}); err != nil {
		log.Fatal(err)
	}
}
