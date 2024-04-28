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

	romPath := os.Args[1]
	rom, err := os.ReadFile(romPath)
	if err != nil {
		fmt.Printf("invalid rom: %s\n", err)
		return
	}
	console, err := gbc.MakeConsole(rom, pl)
	if err != nil {
		fmt.Printf("unable to create the console: %s\n", err)
		return
	}
	savFile := fmt.Sprintf("%s.sav", romPath)
	sav, err := os.ReadFile(savFile)
	if err == nil {
		err = console.LoadSav(sav)
		if err != nil {
			fmt.Printf("unable to load sav: %s\n", err)
			return
		}
	}

	console.Verbose = false
	console.CPU.EnableDisas = false
	console.PrintDebug = false
	err = pl.Run(romPath, console, remote)
	if err != nil {
		fmt.Printf("unable to run the emulator: %s\n", err)
	}

	sav, err = console.StoreSav()
	if err != nil {
		fmt.Printf("unable to store sav: %s\n", err)
		return
	}
	err = os.WriteFile(savFile, sav, 0644)
	if err != nil {
		fmt.Printf("unable to store sav: %s\n", err)
		return
	}
}
