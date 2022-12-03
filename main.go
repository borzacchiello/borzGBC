package main

import (
	"fmt"
	"os"

	"borzGBC/gbc"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("missing ROM filename")
		return
	}

	cart, err := gbc.LoadCartridge(os.Args[1])
	if err != nil {
		fmt.Printf("Unable to load cartridge: %s\n", err)
		return
	}

	// fmt.Printf("%+v\n", cart)
	fmt.Printf("game title: %s\n", cart.GetGameTitle())
}
