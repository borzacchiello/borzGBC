package gbc

type DmaState uint8
type DmaType uint8

const (
	DMA_STATE_INACTIVE DmaState = 0
	DMA_STATE_ACTIVE   DmaState = 1
	DMA_STATE_STARTING DmaState = 2
	DMA_STATE_PAUSED   DmaState = 3
)

const (
	HDMA_TYPE_HDMA DmaType = 0
	HDMA_TYPE_GDMA DmaType = 1
)

type Dma struct {
	GBC *Console

	HdmaWritten     bool
	HdmaState       DmaState
	HdmaType        DmaType
	HdmaControl     uint8
	HdmaSrcHi       uint8
	HdmaSrcLo       uint8
	HdmaDstHi       uint8
	HdmaDstLo       uint8
	HdmaBytesToCopy int
	HdmaHblankBytes int
}

func MakeDma(GBC *Console) *Dma {
	dma := &Dma{
		GBC: GBC,
	}
	return dma
}

func (dma *Dma) HdmaInProgress() bool {
	return dma.HdmaState == DMA_STATE_ACTIVE || dma.HdmaState == DMA_STATE_STARTING
}

func (dma *Dma) DmaRead(addr uint16) uint8 {
	if 0x8000 <= addr && addr < 0xA000 {
		if dma.GBC.PPU.Mode != ACCESS_VRAM && dma.HdmaState != DMA_STATE_ACTIVE {
			return dma.GBC.Read(addr)
		}
		return 0xFF
	}
	if 0xE000 <= addr && addr < 0xFFFF {
		if dma.HdmaState == DMA_STATE_ACTIVE {
			return dma.GBC.Read(addr - 0x4000)
		}
	}
	return dma.GBC.Read(addr)
}

func (dma *Dma) SignalHdma() {
	if dma.HdmaState == DMA_STATE_PAUSED {
		dma.HdmaHblankBytes = 16
		dma.HdmaState = DMA_STATE_STARTING
	}
}

func (dma *Dma) InitHdma() {
	if dma.HdmaControl&0x80 != 0 {
		dma.HdmaType = HDMA_TYPE_HDMA
	} else {
		dma.HdmaType = HDMA_TYPE_GDMA
	}
	dma.HdmaBytesToCopy = (int(dma.HdmaControl&0x7F) + 1) * 16
	dma.HdmaHblankBytes = 16

	dma.HdmaControl &= 0x7F

	if dma.HdmaType == HDMA_TYPE_HDMA && dma.GBC.PPU.Mode != HBLANK {
		dma.HdmaState = DMA_STATE_PAUSED
	} else {
		dma.HdmaState = DMA_STATE_STARTING
	}
}

func (dma *Dma) ExecuteHdma() {
	src := (uint16(dma.HdmaSrcHi) << 8) | uint16(dma.HdmaSrcLo)
	dst := (uint16(dma.HdmaDstHi|0x80) << 8) | uint16(dma.HdmaDstLo)

	len := 2
	if dma.GBC.DoubleSpeedMode {
		len = 1
	}
	if dma.HdmaBytesToCopy < len {
		len = dma.HdmaBytesToCopy
	}

	if dma.HdmaType == HDMA_TYPE_HDMA {
		if dma.HdmaHblankBytes < len {
			len = dma.HdmaHblankBytes
		}
		dma.HdmaHblankBytes -= len
	}
	dma.HdmaBytesToCopy -= len

	for i := 0; i < len; i++ {
		// AFAIK it should be guarded by "dma.GBC.PPU.Mode != ACCESS_VRAM"
		// but some same games do not work...
		dma.GBC.Write(dst, dma.DmaRead(src))

		dst = (dst + 1) & 0x9FFF
		src += 1
	}

	dma.HdmaSrcLo = uint8(src & 0xFF)
	dma.HdmaSrcHi = uint8(src >> 8)
	dma.HdmaDstLo = uint8(dst & 0xFF)
	dma.HdmaDstHi = uint8((dst >> 8) & 0x1F)

	dma.HdmaControl = uint8((dma.HdmaBytesToCopy/16)-1) & 0x7F
}

func (dma *Dma) Step() {
	if dma.HdmaWritten {
		if dma.HdmaState == DMA_STATE_INACTIVE {
			dma.InitHdma()
		} else {
			if dma.HdmaControl&0x80 != 0 {
				dma.InitHdma()
			} else {
				dma.HdmaControl |= 0x80
				dma.HdmaBytesToCopy = 0
				dma.HdmaHblankBytes = 0
				dma.HdmaState = DMA_STATE_INACTIVE
			}
		}
		dma.HdmaWritten = false
	} else if dma.HdmaState == DMA_STATE_STARTING {
		dma.HdmaState = DMA_STATE_ACTIVE
	} else if dma.HdmaState == DMA_STATE_ACTIVE {
		dma.ExecuteHdma()

		if dma.HdmaBytesToCopy == 0 {
			dma.HdmaControl = 0xFF
			dma.HdmaState = DMA_STATE_INACTIVE
		} else if dma.HdmaType == HDMA_TYPE_HDMA && dma.HdmaHblankBytes == 0 {
			dma.HdmaState = DMA_STATE_PAUSED
		}
	}
}

func (dma *Dma) Tick(ticks int) {
	for i := 0; i < ticks; i++ {
		dma.Step()
	}
}
