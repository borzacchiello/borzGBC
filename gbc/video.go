package gbc

type PpuMode int

const (
	ACCESS_OAM  PpuMode = 2
	ACCESS_VRAM         = 3
	HBLANK              = 0
	VBLANK              = 1
)

const (
	CLOCKS_ACCESS_OAM  int = 80
	CLOCKS_ACCESS_VRAM     = 172
	CLOCKS_HBLANK          = 204
	CLOCKS_VBLANK          = 456
)

const (
	SCREEN_WIDTH  int = 160
	SCREEN_HEIGHT int = 144
)

type VideoDriver interface {
	Draw(frameBuffer [SCREEN_HEIGHT][SCREEN_WIDTH]uint8)
}

type Ppu struct {
	Driver VideoDriver

	VRAM_1      [8192]uint8
	VRAM_2      [8192]uint8
	FrameBuffer [SCREEN_HEIGHT][SCREEN_WIDTH]uint8

	LcdStatus, Control uint8

	Mode       PpuMode
	CycleCount int
}

func MakePpu(videoDriver VideoDriver) *Ppu {
	ppu := &Ppu{
		Driver: videoDriver,
	}
	return ppu
}

func (ppu *Ppu) Read(addr uint16) uint8 {
	if addr > 8192 {
		return ppu.VRAM_2[addr-8192]
	}
	return ppu.VRAM_1[addr]
}

func (ppu *Ppu) Write(addr uint16, value uint8) {
	if addr > 8192 {
		ppu.VRAM_2[addr-8192] = value
		return
	}

	ppu.VRAM_1[addr] = value
}

func (ppu *Ppu) Tick(cycles int) {
	ppu.CycleCount += cycles
	switch ppu.Mode {
	case ACCESS_OAM:
		if ppu.CycleCount >= CLOCKS_ACCESS_OAM {
			ppu.LcdStatus |= 3
			ppu.Mode = ACCESS_VRAM
		}
	case ACCESS_VRAM:
	case HBLANK:
	case VBLANK:
	}

	ppu.CycleCount %= CLOCKS_VBLANK
}
