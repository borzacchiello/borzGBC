package main

import (
	"fmt"
	"os"

	"borzGBC/frontend"
	"borzGBC/gbc"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("missing ROM filename")
		return
	}
	remote := ""
	if len(os.Args) > 2 {
		remote = os.Args[2]
		fmt.Printf("remote mode, connecting to %s\n", remote)
	}

	pl, err := frontend.MakeSDLPlugin( /* scaling factor */ 3)
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

	console.Verbose = false
	console.CPU.EnableDisas = false
	console.PrintDebug = false

	err = pl.Run(console, remote)
	if err != nil {
		fmt.Printf("unable to run the emulator: %s\n", err)
	}
}
