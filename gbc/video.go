package gbc

type PpuMode int

const (
	ACCESS_OAM  PpuMode = 2
	ACCESS_VRAM PpuMode = 3
	HBLANK      PpuMode = 0
	VBLANK      PpuMode = 1
)

const (
	CLOCKS_ACCESS_OAM  int = 80
	CLOCKS_ACCESS_VRAM int = 172
	CLOCKS_HBLANK      int = 204
	CLOCKS_VBLANK      int = 456
)

const (
	SCREEN_WIDTH  int = 160
	SCREEN_HEIGHT int = 144
	MAX_SPRITES   int = 10
)

const (
	TILE_SET_ZERO_ADDRESS uint16 = 0x8000
	TILE_SET_ONE_ADDRESS  uint16 = 0x8800
	TILE_MAP_ZERO_ADDRESS uint16 = 0x9800
	TILE_MAP_ONE_ADDRESS  uint16 = 0x9C00
)

const (
	BG_MAP_SIZE    = 256
	TILE_WIDTH_PX  = 8
	TILE_HEIGHT_PX = 8
	TILES_PER_LINE = 32
	TILE_BYTES     = 16
	SPRITE_BYTES   = 4
)

type VideoDriver interface {
	SetPixel(x, y int, color uint32)
	CommitScreen()
}

type Palette struct {
	colors [4]uint32
}

type Tile struct {
	Pixels [8][8]uint8
}

type Sprite struct {
	Ready bool
	x, y  int
	tile  uint8

	options uint8
}

func (sprite *Sprite) gbcPaletteNumber() uint8 {
	return sprite.options & 3
}

func (sprite *Sprite) gbcVramBank() uint8 {
	return (sprite.options >> 3) & 1
}

func (sprite *Sprite) paletteNumber() uint8 {
	return (sprite.options >> 4) & 1
}

func (sprite *Sprite) xFlip() uint8 {
	return (sprite.options >> 5) & 1
}

func (sprite *Sprite) yFlip() uint8 {
	return (sprite.options >> 6) & 1
}

func (sprite *Sprite) renderPriority() uint8 {
	return (sprite.options >> 7) & 1
}

type Ppu struct {
	Driver VideoDriver
	GBC    *Console

	VRAM_1 [0x2000]uint8
	VRAM_2 [0x2000]uint8
	OamRAM [0xA0]uint8

	// VRAM tiles and sprites rearranged here
	tiles   [512]Tile
	sprites [40]Sprite

	STAT, LCDC      uint8
	SCY, SCX        uint8
	LY, LYC, WY, WX uint8
	BGP             uint8
	OBP0, OBP1      uint8

	// A clone of the screen
	screen [SCREEN_WIDTH][SCREEN_HEIGHT]uint8

	Mode       PpuMode
	CycleCount int
	FrameCount int
}

// LCDC Values
func (ppu *Ppu) DisplayEnabled() bool {
	return (ppu.LCDC>>7)&1 != 0
}

func (ppu *Ppu) WindowTileMap() bool {
	return (ppu.LCDC>>6)&1 != 0
}

func (ppu *Ppu) WindowEnabled() bool {
	return (ppu.LCDC>>5)&1 != 0
}

func (ppu *Ppu) BgWindowTileData() bool {
	return (ppu.LCDC>>4)&1 != 0
}

func (ppu *Ppu) BgTileMapDisplay() bool {
	return (ppu.LCDC>>3)&1 != 0
}

func (ppu *Ppu) SpriteSize() bool {
	return (ppu.LCDC>>2)&1 != 0
}

func (ppu *Ppu) SpritesEnabled() bool {
	return (ppu.LCDC>>1)&1 != 0
}

func (ppu *Ppu) BgEnabled() bool {
	return ppu.LCDC&1 != 0
}

// STAT Values
func (ppu *Ppu) modeFlag() uint8 {
	return ppu.STAT & 3
}

func (ppu *Ppu) coincidenceFlag() bool {
	return (ppu.STAT>>2)&1 != 0
}

func (ppu *Ppu) hblankInterrupt() bool {
	return (ppu.STAT>>3)&1 != 0
}

func (ppu *Ppu) vblankInterrupt() bool {
	return (ppu.STAT>>4)&1 != 0
}

func (ppu *Ppu) oamInterrupt() bool {
	return (ppu.STAT>>5)&1 != 0
}

func (ppu *Ppu) coincidenceInterrupt() bool {
	return (ppu.STAT>>6)&1 != 0
}

func MakePpu(GBC *Console, videoDriver VideoDriver) *Ppu {
	ppu := &Ppu{
		Driver: videoDriver,
		GBC:    GBC,
		Mode:   ACCESS_OAM,
	}
	return ppu
}

func (ppu *Ppu) setPixel(x, y int, c uint8, palette *Palette) {
	color := palette.colors[c]

	ppu.screen[x][y] = c
	ppu.Driver.SetPixel(x, y, color)
}

func (ppu *Ppu) ReadVRam(addr uint16) uint8 {
	if addr >= 0x2000 {
		return ppu.VRAM_2[addr-0x2000]
	}
	return ppu.VRAM_1[addr]
}

func (ppu *Ppu) WriteVRam(addr uint16, value uint8) {
	if addr > 0x2000 {
		ppu.VRAM_2[addr-0x2000] = value
	} else {
		ppu.VRAM_1[addr] = value
	}

	// Update tiles metadata
	// It is not strictly needed, but helps readability during rendering process
	addr &= 0xfffe
	tile := (addr >> 4) & 0x1ff
	y := (addr >> 1) & 7

	for x := uint8(0); x < 8; x++ {
		bitIndex := uint8(1 << (7 - x))
		v := uint8(0)
		if ppu.ReadVRam(addr)&bitIndex != 0 {
			v += 1
		}
		if ppu.ReadVRam(addr+1)&bitIndex != 0 {
			v += 2
		}
		ppu.tiles[tile].Pixels[y][x] = v
	}
}

func (ppu *Ppu) ReadOam(addr uint16) uint8 {
	return ppu.OamRAM[addr]
}

func (ppu *Ppu) WriteOam(addr uint16, value uint8) {
	ppu.OamRAM[addr] = value

	// Update sprites metadata
	// It is not strictly needed, but helps readability during rendering process
	sprite := &ppu.sprites[addr>>2]
	sprite.Ready = false
	switch addr & 3 {
	case 0:
		sprite.y = int(value) - 16
	case 1:
		sprite.x = int(value) - 8
	case 2:
		sprite.tile = value
	case 3:
		sprite.Ready = true
		sprite.options = value
	}
}

func (ppu *Ppu) setMode(mode PpuMode) {
	ppu.Mode = mode & 3
	ppu.STAT &= ^uint8(3)
	ppu.STAT |= uint8(ppu.Mode)
}

func (ppu *Ppu) setCoincidenceFlag(value bool) {
	ppu.STAT &= ^uint8(4)
	if value {
		ppu.STAT |= 4
	}
}

func getRGBFromColor(c uint8) uint32 {
	switch c {
	case 0:
		// White
		return 0xFFFFFFFF
	case 1:
		// Light Grey
		return 0xAAAAAAFF
	case 2:
		// Dark Grey
		return 0x555555FF
	case 3:
		// Black
		return 0x000000FF
	default:
		panic("getRGBFromColor(): invalid color")
	}
}

func (ppu *Ppu) loadPalette(reg uint8) Palette {
	var c0, c1, c2, c3 uint8
	c0 = reg & 3
	c1 = (reg >> 2) & 3
	c2 = (reg >> 4) & 3
	c3 = (reg >> 6) & 3

	return Palette{colors: [4]uint32{
		getRGBFromColor(c0),
		getRGBFromColor(c1),
		getRGBFromColor(c2),
		getRGBFromColor(c3)}}
}

func getPixelColor(p1, p2 uint8, tile_pixel int) uint8 {
	return ((p1>>(7-tile_pixel))&1)<<1 | (p2>>(7-tile_pixel))&1
}

func (ppu *Ppu) drawBgLine() {
	addr := TILE_MAP_ZERO_ADDRESS
	if ppu.BgTileMapDisplay() {
		addr = TILE_MAP_ONE_ADDRESS
	}

	palette := ppu.loadPalette(ppu.BGP)
	useTileSetZero := ppu.BgWindowTileData()
	addr += ((uint16(ppu.SCY) + uint16(ppu.LY)) / 8 * 32) % 1024

	startRowAddr := addr
	endRowAddr := addr + 32
	addr += uint16(ppu.SCX) >> 3

	screen_x := 0
	screen_y := int(ppu.LY)

	x := ppu.SCX & 7
	y := (ppu.SCY + ppu.LY) & 7
	for i := uint16(0); i < 21; i++ {
		tileAddr := addr + i
		if tileAddr >= endRowAddr {
			tileAddr = startRowAddr + tileAddr%endRowAddr
		}

		tile := int(ppu.GBC.Read(tileAddr))
		if !useTileSetZero && tile < 128 {
			tile += 256
		}

		for ; x < 8; x++ {
			if screen_x >= SCREEN_WIDTH {
				break
			}

			color := ppu.tiles[tile].Pixels[y][x]
			ppu.setPixel(screen_x, screen_y, color, &palette)
			screen_x++
		}
		x = 0
	}
}

func (ppu *Ppu) drawWindowLine() {
	if ppu.WY > ppu.LY {
		return
	}

	addr := TILE_MAP_ZERO_ADDRESS
	if ppu.WindowTileMap() {
		addr = TILE_MAP_ONE_ADDRESS
	}

	palette := ppu.loadPalette(ppu.BGP)
	useTileSetZero := ppu.BgWindowTileData()
	addr += ((uint16(ppu.LY) - uint16(ppu.WY)) / 8) * 32

	y := (uint16(ppu.LY) - uint16(ppu.WY)) & 7

	screen_x := int(ppu.WX) - 7
	screen_y := int(ppu.LY)

	for tileAddr := addr; tileAddr < addr+20; tileAddr++ {
		tile := int(ppu.GBC.Read(tileAddr))
		if !useTileSetZero && tile < 128 {
			tile += 256
		}

		for x := 0; x < 8; x++ {
			if screen_x >= SCREEN_WIDTH {
				continue
			}

			color := ppu.tiles[tile].Pixels[y][x]
			ppu.setPixel(screen_x, screen_y, color, &palette)
			screen_x++
		}
	}
}

func (ppu *Ppu) drawSprites() {
	spriteHeight := 8
	if ppu.SpriteSize() {
		spriteHeight = 16
	}

	renderedSprites := 0
	for i := 39; i >= 0; i-- {
		sprite := &ppu.sprites[i]
		if !sprite.Ready {
			continue
		}
		if (sprite.y > int(ppu.LY)) || (sprite.y+spriteHeight) <= int(ppu.LY) {
			continue
		}

		if renderedSprites >= MAX_SPRITES {
			continue
		}
		renderedSprites++

		if (sprite.x < -7) || (sprite.x >= 160) {
			continue
		}

		pixelY := int(ppu.LY) - sprite.y
		if sprite.yFlip() != 0 {
			off := 0
			if ppu.SpriteSize() {
				off = 8
			}
			pixelY = (7 + off) - pixelY
		}

		screen_x := 0
		screen_y := int(ppu.LY)

		for x := 0; x < 8; x++ {
			tileNum := sprite.tile
			if ppu.SpriteSize() {
				tileNum &= 0xFE
			}

			screen_x = sprite.x + x
			if screen_x < 0 || screen_x >= SCREEN_WIDTH {
				continue
			}

			pixelX := x
			if sprite.xFlip() != 0 {
				pixelX = 7 - x
			}

			palette := ppu.loadPalette(ppu.OBP0)
			if sprite.paletteNumber() != 0 {
				palette = ppu.loadPalette(ppu.OBP1)
			}

			color := uint8(0)
			if ppu.SpriteSize() && pixelY >= 8 {
				color = ppu.tiles[tileNum+1].Pixels[pixelY-8][pixelX]
			} else {
				color = ppu.tiles[tileNum].Pixels[pixelY][pixelX]
			}
			if color == 0 {
				continue
			}
			if ppu.screen[screen_x][screen_y] == 0 || sprite.renderPriority() == 0 {
				ppu.setPixel(screen_x, screen_y, color, &palette)
			}
		}
	}
}

func (ppu *Ppu) writeScanline() {
	if !ppu.DisplayEnabled() {
		return
	}

	if ppu.BgEnabled() {
		ppu.drawBgLine()
	}

	if ppu.WindowEnabled() {
		ppu.drawWindowLine()
	}

	if ppu.SpritesEnabled() {
		ppu.drawSprites()
	}
}

func (ppu *Ppu) checkCoincidenceLY_LYC() {
	ppu.setCoincidenceFlag(ppu.LYC == ppu.LY)

	if ppu.coincidenceFlag() && ppu.coincidenceInterrupt() {
		ppu.GBC.CPU.SetInterrupt(InterruptLCDStat.Mask)
	}
}

func (ppu *Ppu) Tick(cycles int) {
	ppu.CycleCount += cycles

	if !ppu.DisplayEnabled() {
		ppu.CycleCount = 0
		ppu.LY = 0
		ppu.setMode(ACCESS_OAM)
		return
	}

	switch ppu.Mode {
	case ACCESS_OAM:
		if ppu.CycleCount >= CLOCKS_ACCESS_OAM {
			ppu.CycleCount %= CLOCKS_ACCESS_OAM
			ppu.setMode(ACCESS_VRAM)
		}
	case ACCESS_VRAM:
		if ppu.CycleCount >= CLOCKS_ACCESS_VRAM {
			ppu.CycleCount %= CLOCKS_ACCESS_VRAM
			ppu.setMode(HBLANK)

			ppu.writeScanline()

			if ppu.hblankInterrupt() {
				ppu.GBC.CPU.SetInterrupt(InterruptLCDStat.Mask)
			}
		}
	case HBLANK:
		if ppu.CycleCount >= CLOCKS_HBLANK {
			ppu.CycleCount %= CLOCKS_HBLANK

			ppu.LY += 1
			ppu.checkCoincidenceLY_LYC()

			if ppu.LY == 144 {
				ppu.setMode(VBLANK)

				ppu.Driver.CommitScreen()
				ppu.FrameCount += 1

				ppu.GBC.CPU.SetInterrupt(InterruptVBlank.Mask)
				if ppu.vblankInterrupt() {
					ppu.GBC.CPU.SetInterrupt(InterruptLCDStat.Mask)
				}
			} else {
				ppu.setMode(ACCESS_OAM)
				if ppu.oamInterrupt() {
					ppu.GBC.CPU.SetInterrupt(InterruptLCDStat.Mask)
				}
			}
		}
	case VBLANK:
		if ppu.CycleCount >= CLOCKS_VBLANK {
			ppu.CycleCount %= CLOCKS_VBLANK

			ppu.LY += 1
			ppu.checkCoincidenceLY_LYC()

			if ppu.LY == 153 {
				ppu.LY = 0
				ppu.setMode(ACCESS_OAM)
				if ppu.oamInterrupt() {
					ppu.GBC.CPU.SetInterrupt(InterruptLCDStat.Mask)
				}
			}
		}
	}
}
