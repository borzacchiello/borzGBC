package frontend

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

type ImageVideoDriver struct {
	backImg  *image.RGBA
	frontImg *image.RGBA
	num      int
}

func MkImageVideoDriver() *ImageVideoDriver {
	res := &ImageVideoDriver{}

	upLeft := image.Point{0, 0}
	lowRight := image.Point{160, 144}
	res.backImg = image.NewRGBA(image.Rectangle{upLeft, lowRight})
	res.frontImg = image.NewRGBA(image.Rectangle{upLeft, lowRight})

	return res
}

func (d *ImageVideoDriver) SetPixel(x, y int, c uint32) {
	var r, g, b, a uint8
	r = uint8((c >> 24) & 0xFF)
	g = uint8((c >> 16) & 0xFF)
	b = uint8((c >> 8) & 0xFF)
	a = uint8(c & 0xFF)

	d.backImg.SetRGBA(x, y, color.RGBA{r, g, b, a})
}

func (d *ImageVideoDriver) CommitScreen() {
	d.frontImg = d.backImg
	d.num += 1
}

func (d *ImageVideoDriver) SaveScreen(path string) {
	f, _ := os.Create(path)
	defer f.Close()

	png.Encode(f, d.frontImg)
}

func (pl *ImageVideoDriver) NotifyAudioSample(l, r int8) {
	// Ignore audio
}

func (pl *ImageVideoDriver) GetCurrentImage() *image.RGBA {
	return pl.frontImg
}
