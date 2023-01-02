package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
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

func runRomTest(t *testing.T, test string, frames int) {
	testName := strings.Split(test, ".")[0]
	pl := mediaplugin.MkImageVideoDriver()
	console, err := gbc.MakeConsole(fmt.Sprintf("testRoms/%s", test), pl)
	if err != nil {
		t.Error("Unable to create console")
		return
	}
	defer console.Destroy()

	for console.PPU.FrameCount < frames {
		console.Step()
	}

	pl.SaveScreen(fmt.Sprintf("testRoms/%s.result.png", testName))

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

	if !imagesAreEqual(expected, pl.FrontImg) {
		t.Fail()
	}
}

func TestBlargCpuInstrs(t *testing.T) {
	runRomTest(t, "Blargg/cpu_instrs.gb", 4000)
}

func TestBlargInstrTiming(t *testing.T) {
	runRomTest(t, "Blargg/instr_timing.gb", 1000)
}

func TestDMGAcid2(t *testing.T) {
	runRomTest(t, "MattCurrie/dmg-acid2.gb", 1000)
}

func TestCGBAcid2(t *testing.T) {
	runRomTest(t, "MattCurrie/cgb-acid2.gbc", 1000)
}

func TestMooneyeOamDmaBasic(t *testing.T) {
	runRomTest(t, "Mooneye/oam_dma/basic.gb", 1000)
}

func TestMooneyeOamDmaRegRead(t *testing.T) {
	runRomTest(t, "Mooneye/oam_dma/reg_read.gb", 1000)
}
