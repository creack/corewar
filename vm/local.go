//go:build !local

package main

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func init() {
	go func() {
		time.Sleep(100 * time.Millisecond)
		ebiten.SetWindowPosition(-initialScreenWidth, 0)
	}()
}
