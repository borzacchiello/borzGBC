package main

import (
	"fmt"
	"os"

	"borzGBC/gbc"
)

type VoidVideoDriver struct {
}

func (d *VoidVideoDriver) Draw(frameBuffer [gbc.SCREEN_HEIGHT][gbc.SCREEN_WIDTH]uint8) {
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("missing ROM filename")
		return
	}

	vd := &VoidVideoDriver{}
	console, err := gbc.MakeConsole(os.Args[1], vd)
	if err != nil {
		fmt.Printf("Unable to load cartridge: %s\n", err)
		return
	}

	console.Run()
}
