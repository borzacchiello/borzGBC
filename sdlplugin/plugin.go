package sdlplugin

import (
	"borzGBC/gbc"
	"fmt"

	"github.com/veandco/go-sdl2/sdl"
)

type SDLPlugin struct {
	Window        *sdl.Window
	Renderer      *sdl.Renderer
	Width, Height int
	Scale         int
}

func MakeSDLPlugin(scale int) (*SDLPlugin, error) {
	var err error

	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		return nil, err
	}
	pl := &SDLPlugin{
		Width:  160,
		Height: 144,
		Scale:  scale,
	}

	pl.Window, err = sdl.CreateWindow(
		"BorzGBC",
		sdl.WINDOWPOS_UNDEFINED,
		sdl.WINDOWPOS_UNDEFINED,
		int32(pl.Width), int32(pl.Height),
		sdl.WINDOW_SHOWN)
	if err != nil {
		return nil, err
	}

	pl.Renderer, err = sdl.CreateRenderer(
		pl.Window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		return nil, err
	}
	fmt.Printf("renderer: %p\n", pl.Renderer)

	pl.Renderer.SetDrawColor(0, 0, 0, 255)
	pl.Renderer.Clear()
	return pl, nil
}

func (pl *SDLPlugin) Destroy() {
	pl.Renderer.Destroy()
	pl.Window.Destroy()
	sdl.Quit()
}

func (pl *SDLPlugin) SetPixel(x, y int, c uint32) {
	var r, g, b, a uint8
	r = uint8((c >> 24) & 0xFF)
	g = uint8((c >> 16) & 0xFF)
	b = uint8((c >> 8) & 0xFF)
	a = uint8(c & 0xFF)

	pl.Renderer.SetDrawColor(r, g, b, a)
	pl.Renderer.DrawPoint(int32(x), int32(y))

}

func (pl *SDLPlugin) CommitScreen() {
	pl.Renderer.Present()
}

func (pl *SDLPlugin) Run(romFilename string) {
	console, err := gbc.MakeConsole(romFilename, pl)
	if err != nil {
		fmt.Printf("Unable to load cartridge: %s\n", err)
		return
	}
	console.CPU.EnableDisas = false

	running := true
	for running {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event.(type) {
			case *sdl.QuitEvent:
				println("Quit")
				running = false
				break
			}
		}

		cycles := console.Step()
		sdl.Delay(uint32(console.GetMs(cycles)))
	}
}
