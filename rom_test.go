package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"

	"borzGBC/gbc"
	"borzGBC/mediaplugin"
)

func imagesAreEqual(img1, img2 image.Image) bool {
	if img1.Bounds() != img2.Bounds() {
		return false
	}

	for x := img1.Bounds().Min.X; x < img1.Bounds().Max.X; x++ {
		for y := img1.Bounds().Min.Y; y < img1.Bounds().Max.Y; y++ {
			if img1.At(x, y) != img2.At(x, y) {
				return false
			}
		}
	}
	return true
}

func runRomTest(t *testing.T, testName string, frames int) {
	pl := mediaplugin.MkImageVideoDriver()
	console, err := gbc.MakeConsole(fmt.Sprintf("testRoms/%s.gb", testName), pl)
	if err != nil {
		t.Error("Unable to create console")
		return
	}
	defer console.Destroy()

	f, err := os.Open(fmt.Sprintf("testRoms/%s.png", testName))
	if err != nil {
		t.Error("Unable to read expected screen")
		return
	}
	defer f.Close()
	expected, err := png.Decode(f)
	if err != nil {
		t.Error("Unable to decode expected screen")
		return
	}

	for console.PPU.FrameCount < frames {
		console.Step()
	}

	pl.SaveScreen(fmt.Sprintf("testRoms/%s.result.png", testName))
	if !imagesAreEqual(expected, pl.FrontImg) {
		t.Fail()
	}

}

func TestBlargCpuInstrs(t *testing.T) {
	runRomTest(t, "cpu_instrs", 4000)
}

func TestBlargInstrTiming(t *testing.T) {
	runRomTest(t, "instr_timing", 1000)
}

func TestDMGAcid2(t *testing.T) {
	t.Skip("Broken")

	runRomTest(t, "dmg-acid2", 1000)
}
