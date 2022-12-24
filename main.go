package main

import (
	"fmt"
	"os"

	"borzGBC/gbc"
	"borzGBC/mediaplugin"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("missing ROM filename")
		return
	}

	pl, err := mediaplugin.MakeSDLPlugin(3)
	if err != nil {
		fmt.Printf("unable to create SDLPlugin: %s\n", err)
		return
	}
	defer pl.Destroy()

	console, err := gbc.MakeConsole(os.Args[1], pl)
	if err != nil {
		fmt.Printf("unable to create the console: %s\n", err)
		return
	}
	defer console.Destroy()

	console.CPU.EnableDisas = false
	console.PrintDebug = false

	err = pl.Run(console)
	if err != nil {
		fmt.Printf("unable to run the emulator: %s\n", err)
	}
}
