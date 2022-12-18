package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"borzGBC/sdlplugin"
)

type ImageVideoDriver struct {
	img *image.RGBA
	num int
}

func MkImageVideoDriver() *ImageVideoDriver {
	res := &ImageVideoDriver{}

	upLeft := image.Point{0, 0}
	lowRight := image.Point{160, 144}
	res.img = image.NewRGBA(image.Rectangle{upLeft, lowRight})

	return res
}

func (d *ImageVideoDriver) SetPixel(x, y int, c uint32) {
	var r, g, b, a uint8
	r = uint8((c >> 24) & 0xFF)
	g = uint8((c >> 16) & 0xFF)
	b = uint8((c >> 8) & 0xFF)
	a = uint8(c & 0xFF)

	d.img.SetRGBA(x, y, color.RGBA{r, g, b, a})
}
func (d *ImageVideoDriver) CommitScreen() {
	f, _ := os.Create(fmt.Sprintf("/tmp/img%03d.png", d.num))
	png.Encode(f, d.img)

	d.num += 1
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("missing ROM filename")
		return
	}

	// vd := MkImageVideoDriver()
	pl, err := sdlplugin.MakeSDLPlugin(1)
	if err != nil {
		fmt.Printf("unable to create SDLPlugin: %s\n", err)
	}

	pl.Run(os.Args[1])
}
