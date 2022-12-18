package main

import (
	"fmt"
	"os"

	"borzGBC/sdlplugin"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("missing ROM filename")
		return
	}

	pl, err := sdlplugin.MakeSDLPlugin(3)
	if err != nil {
		fmt.Printf("unable to create SDLPlugin: %s\n", err)
	}
	defer pl.Destroy()

	err = pl.Run(os.Args[1])
	if err != nil {
		fmt.Printf("unable to run the emulator: %s\n", err)
	}
}
