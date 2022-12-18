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

	pl, err := sdlplugin.MakeSDLPlugin(1)
	if err != nil {
		fmt.Printf("unable to create SDLPlugin: %s\n", err)
	}

	pl.Run(os.Args[1])
}
