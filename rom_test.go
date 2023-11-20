package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"testing"

	"borzGBC/frontend"
	"borzGBC/gbc"
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
	pl := frontend.MkImageVideoDriver()
	console, err := gbc.MakeConsole(fmt.Sprintf("testRoms/%s", test), pl)
	if err != nil {
		t.Error("Unable to create console")
		return
	}
	defer console.Destroy()

	console.PPU.GBPalette = gbc.GB_PALETTE_GREY
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

	if !imagesAreEqual(expected, pl.GetCurrentImage()) {
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

func TestMooneyeOamDma_basic(t *testing.T) {
	runRomTest(t, "Mooneye/oam_dma/basic.gb", 1000)
}

func TestMooneyeOamDma_reg_read(t *testing.T) {
	runRomTest(t, "Mooneye/oam_dma/reg_read.gb", 1000)
}

func TestMooneyeBits_mem_oam(t *testing.T) {
	runRomTest(t, "Mooneye/bits/mem_oam.gb", 1000)
}

func TestMooneyeBits_reg_f(t *testing.T) {
	runRomTest(t, "Mooneye/bits/reg_f.gb", 1000)
}

func TestMooneye_sprite_priority(t *testing.T) {
	runRomTest(t, "Mooneye/sprite_priority.gb", 1000)
}

func TestMooneyeTimer_div_write(t *testing.T) {
	runRomTest(t, "Mooneye/timer/div_write.gb", 1000)
}

func TestMooneyeTimer_tim00(t *testing.T) {
	runRomTest(t, "Mooneye/timer/tim00.gb", 1000)
}

func TestMooneyeTimer_tim10(t *testing.T) {
	runRomTest(t, "Mooneye/timer/tim10.gb", 1000)
}

func TestMooneyeTimer_tim11(t *testing.T) {
	runRomTest(t, "Mooneye/timer/tim11.gb", 1000)
}

func TestMooneyeMbc1_bits_bank1(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/bits_bank1.gb", 1000)
}

func TestMooneyeMbc1_bits_bank2(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/bits_bank2.gb", 1000)
}

func TestMooneyeMbc1_bits_mode(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/bits_mode.gb", 1000)
}

func TestMooneyeMbc1_bits_ramg(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/bits_ramg.gb", 1000)
}

func TestMooneyeMbc1_ram_64kb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/ram_64kb.gb", 1000)
}

func TestMooneyeMbc1_ram_256kb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/ram_256kb.gb", 1000)
}

func TestMooneyeMbc1_rom_1Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/rom_1Mb.gb", 1000)
}

func TestMooneyeMbc1_rom_2Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/rom_2Mb.gb", 1000)
}

func TestMooneyeMbc1_rom_4Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/rom_4Mb.gb", 1000)
}

func TestMooneyeMbc1_rom_8Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/rom_8Mb.gb", 1000)
}

func TestMooneyeMbc1_rom_16Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/rom_16Mb.gb", 1000)
}

func TestMooneyeMbc1_rom_512Kb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc1/rom_512kb.gb", 1000)
}

func TestMooneyeMbc5_rom_1Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc5/rom_1Mb.gb", 1000)
}

func TestMooneyeMbc5_rom_2Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc5/rom_2Mb.gb", 1000)
}

func TestMooneyeMbc5_rom_4Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc5/rom_4Mb.gb", 1000)
}

func TestMooneyeMbc5_rom_8Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc5/rom_8Mb.gb", 1000)
}

func TestMooneyeMbc5_rom_16Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc5/rom_16Mb.gb", 1000)
}

func TestMooneyeMbc5_rom_32Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc5/rom_32Mb.gb", 1000)
}

func TestMooneyeMbc5_rom_64Mb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc5/rom_64Mb.gb", 1000)
}

func TestMooneyeMbc5_rom_512Kb(t *testing.T) {
	runRomTest(t, "Mooneye/mbc5/rom_512kb.gb", 1000)
}

func TestMooneyeIntr_1_2(t *testing.T) {
	runRomTest(t, "Mooneye/interrupts/intr_1_2_timing-GS.gb", 1000)
}

func TestMooneyeIntr_2_0(t *testing.T) {
	runRomTest(t, "Mooneye/interrupts/intr_2_0_timing.gb", 1000)
}

func TestMooneyeIntrStatIrq(t *testing.T) {
	// This fails, but it proceeds further with respect to the previous version
	runRomTest(t, "Mooneye/interrupts/stat_irq_blocking.gb", 1000)
}
