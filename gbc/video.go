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

type Ppu struct {
	Driver VideoDriver
	GBC    *Console

	VRAM_1 [8192]uint8
	VRAM_2 [8192]uint8

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

	ppu.Driver.SetPixel(x, y, color)
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
	c0 = ((reg>>1)&1)<<1 | (reg>>0)&1
	c1 = ((reg>>3)&1)<<1 | (reg>>2)&1
	c2 = ((reg>>5)&1)<<1 | (reg>>4)&1
	c3 = ((reg>>7)&1)<<1 | (reg>>6)&1

	return Palette{colors: [4]uint32{
		getRGBFromColor(c0),
		getRGBFromColor(c1),
		getRGBFromColor(c2),
		getRGBFromColor(c3)}}
}

func getPixelColor(p1, p2 uint8, tile_pixel int) uint8 {
	return ((p1>>(7-tile_pixel))&1)<<1 | (p2>>(7-tile_pixel))&1
}

func (ppu *Ppu) drawBgLine(line uint8) {
	palette := ppu.loadPalette(ppu.BGP)

	tile_set_addr := TILE_SET_ZERO_ADDRESS
	if !ppu.BgWindowTileData() {
		tile_set_addr = TILE_SET_ONE_ADDRESS
	}
	tile_map_addr := TILE_MAP_ZERO_ADDRESS
	if ppu.BgTileMapDisplay() {
		tile_map_addr = TILE_MAP_ONE_ADDRESS
	}

	screen_y := int(line)
	for screen_x := 0; screen_x < SCREEN_WIDTH; screen_x++ {
		scrolled_x := screen_x + int(ppu.SCX)
		scrolled_y := screen_y + int(ppu.SCY)

		bg_map_x := scrolled_x % BG_MAP_SIZE
		bg_map_y := scrolled_y % BG_MAP_SIZE

		tile_x := bg_map_x / TILE_WIDTH_PX
		tile_y := bg_map_y / TILE_HEIGHT_PX

		tile_pixel_x := bg_map_x % TILE_WIDTH_PX
		tile_pixel_y := bg_map_y % TILE_HEIGHT_PX

		tile_index := tile_y*TILES_PER_LINE + tile_x
		tile_id_addr := tile_map_addr + uint16(tile_index)

		tile_id := ppu.GBC.Read(tile_id_addr)

		tile_data_mem_off := int(tile_id) * TILE_BYTES
		if !ppu.BgWindowTileData() {
			tile_data_mem_off = (int(tile_id) + 128) * TILE_BYTES
		}

		tile_data_line_off := tile_pixel_y * 2
		tile_line_data_start_addr := tile_set_addr + uint16(tile_data_mem_off) + uint16(tile_data_line_off)

		pixel_1 := ppu.GBC.Read(tile_line_data_start_addr)
		pixel_2 := ppu.GBC.Read(tile_line_data_start_addr + 1)

		pixel_color := getPixelColor(pixel_1, pixel_2, tile_pixel_x)
		ppu.setPixel(screen_x, screen_y, pixel_color, &palette)
	}
}

func (ppu *Ppu) drawWindowLine(line uint8) {
	palette := ppu.loadPalette(ppu.BGP)

	tile_set_addr := TILE_SET_ZERO_ADDRESS
	if !ppu.BgWindowTileData() {
		tile_set_addr = TILE_SET_ONE_ADDRESS
	}
	tile_map_addr := TILE_MAP_ZERO_ADDRESS
	if ppu.WindowTileMap() {
		tile_map_addr = TILE_MAP_ONE_ADDRESS
	}

	screen_y := int(line)
	scrolled_y := screen_y - int(ppu.WY)

	for screen_x := 0; screen_x < SCREEN_WIDTH; screen_x++ {
		scrolled_x := screen_x + int(ppu.WX) - 7

		tile_x := scrolled_x / TILE_WIDTH_PX
		tile_y := scrolled_y / TILE_HEIGHT_PX

		tile_pixel_x := scrolled_x % TILE_WIDTH_PX
		tile_pixel_y := scrolled_y % TILE_HEIGHT_PX

		tile_index := tile_y*TILES_PER_LINE + tile_x
		tile_id_addr := tile_map_addr + uint16(tile_index)

		tile_id := ppu.GBC.Read(tile_id_addr)

		tile_data_mem_off := int(tile_id) * TILE_BYTES
		if ppu.BgWindowTileData() {
			tile_data_mem_off = (int(tile_id) + 128) * TILE_BYTES
		}

		tile_data_line_off := tile_pixel_y * 2
		tile_line_data_start_addr := tile_set_addr + uint16(tile_data_mem_off) + uint16(tile_data_line_off)

		pixel_1 := ppu.GBC.Read(tile_line_data_start_addr)
		pixel_2 := ppu.GBC.Read(tile_line_data_start_addr + 1)

		pixel_color := getPixelColor(pixel_1, pixel_2, tile_pixel_x)
		ppu.setPixel(screen_x, screen_y, pixel_color, &palette)
	}
}

type Tile struct {
	buffer [TILE_HEIGHT_PX * 2 * TILE_WIDTH_PX]uint8
}

func pixelIndex(x, y int) int {
	return y*TILE_HEIGHT_PX + x
}

func makeTile(GBC *Console, addr uint16, mult int) *Tile {
	res := &Tile{}

	for x := 0; x < TILE_WIDTH_PX; x++ {
		for y := 0; y < TILE_HEIGHT_PX*mult; y++ {
			res.buffer[pixelIndex(x, y)] = 0
		}
	}

	for tile_line := 0; tile_line < TILE_HEIGHT_PX*mult; tile_line++ {
		index_into_tile := 2 * tile_line
		line_start := addr + uint16(index_into_tile)

		p1 := GBC.Read(line_start)
		p2 := GBC.Read(line_start + 1)

		for x := 0; x < TILE_WIDTH_PX; x++ {
			v := ((p2>>(7-x))&1)<<1 | (p1>>(7-x))&1
			res.buffer[pixelIndex(x, tile_line)] = v
		}
	}
	return res
}

func (t *Tile) getPixel(x, y int) uint8 {
	return t.buffer[pixelIndex(x, y)]
}

func (ppu *Ppu) drawSprite(sprite_id int) {
	offInOam := uint16(sprite_id * SPRITE_BYTES)
	oamStart := 0xFE00 + offInOam

	sprite_y := ppu.GBC.Read(oamStart)
	sprite_x := ppu.GBC.Read(oamStart + 1)

	if sprite_y == 0 || sprite_y >= 160 {
		return
	}
	if sprite_x == 0 || sprite_x >= 168 {
		return
	}

	sprite_multiplier := 1
	if ppu.SpriteSize() {
		sprite_multiplier = 2
	}

	tile_set_location := TILE_SET_ZERO_ADDRESS

	pattern_n := ppu.GBC.Read(oamStart + 2)
	sprite_attrs := ppu.GBC.Read(oamStart + 3)

	use_palette_1 := sprite_attrs&(1<<4) != 0
	flip_x := sprite_attrs&(1<<5) != 0
	flip_y := sprite_attrs&(1<<6) != 0
	obj_behind_bg := sprite_attrs&(1<<7) != 0

	palette := ppu.loadPalette(ppu.OBP0)
	if use_palette_1 {
		palette = ppu.loadPalette(ppu.OBP1)
	}

	tile_off := pattern_n * TILE_BYTES

	pattern_addr := tile_set_location + uint16(tile_off)

	tile := makeTile(ppu.GBC, pattern_addr, sprite_multiplier)
	start_y := sprite_y - 16
	start_x := sprite_x - 8

	for y := 0; y < TILE_HEIGHT_PX*sprite_multiplier; y++ {
		for x := 0; x < TILE_WIDTH_PX; x++ {
			maybe_flipped_y := y
			if flip_y {
				maybe_flipped_y = TILE_HEIGHT_PX*sprite_multiplier - y - 1
			}
			maybe_flipped_x := x
			if flip_x {
				maybe_flipped_x = TILE_WIDTH_PX - x - 1
			}

			color := tile.getPixel(maybe_flipped_x, maybe_flipped_y)
			if color == 0 {
				continue
			}

			screen_x := int(start_x) + x
			screen_y := int(start_y) + y
			if screen_x >= SCREEN_WIDTH || screen_y >= SCREEN_HEIGHT {
				continue
			}

			if ppu.screen[screen_x][screen_y] != 0 && obj_behind_bg {
				continue
			}

			ppu.setPixel(screen_x, screen_y, color, &palette)
		}
	}
}

func (ppu *Ppu) writeScanline(line uint8) {
	if !ppu.DisplayEnabled() {
		return
	}

	if ppu.BgEnabled() {
		ppu.drawBgLine(line)
	}

	if ppu.WindowEnabled() {
		ppu.drawWindowLine(line)
	}
}

func (ppu *Ppu) writeSprites() {
	if !ppu.SpritesEnabled() {
		return
	}

	for i := 0; i < 40; i++ {
		ppu.drawSprite(i)
	}
}

func (ppu *Ppu) Tick(cycles int) {
	ppu.CycleCount += cycles
	switch ppu.Mode {
	case ACCESS_OAM:
		if ppu.CycleCount >= CLOCKS_ACCESS_OAM {
			ppu.CycleCount %= CLOCKS_ACCESS_OAM
			ppu.Mode = ACCESS_VRAM

			ppu.STAT |= 3
		}
	case ACCESS_VRAM:
		if ppu.CycleCount >= CLOCKS_ACCESS_VRAM {
			ppu.CycleCount %= CLOCKS_ACCESS_VRAM
			ppu.Mode = HBLANK

			lyConincidence := ppu.LY == ppu.LYC
			if (ppu.STAT&4 != 0) || (ppu.STAT&64 != 0 && lyConincidence) {
				ppu.GBC.CPU.SetInterrupt(InterruptLCDStat.Mask)
			}

			ppu.STAT &= ^uint8(7)
			if lyConincidence {
				ppu.STAT |= 4
			}
		}
	case HBLANK:
		if ppu.CycleCount >= CLOCKS_HBLANK {
			ppu.CycleCount %= CLOCKS_HBLANK

			ppu.writeScanline(ppu.LY)

			ppu.LY += 1
			if ppu.LY == 144 {
				ppu.Mode = VBLANK
				ppu.STAT &= ^uint8(2)
				ppu.STAT |= 1
				ppu.GBC.CPU.SetInterrupt(InterruptVBlank.Mask)
			} else {
				ppu.Mode = ACCESS_OAM
				ppu.STAT |= 2
				ppu.STAT &= ^uint8(1)
			}
		}
	case VBLANK:
		if ppu.CycleCount >= CLOCKS_VBLANK {
			ppu.CycleCount %= CLOCKS_VBLANK

			ppu.LY += 1
			if ppu.LY == 154 {
				ppu.writeSprites()
				ppu.Driver.CommitScreen()
				ppu.FrameCount += 1

				ppu.LY = 0
				ppu.Mode = ACCESS_OAM
				ppu.STAT |= 2
				ppu.STAT &= ^uint8(1)
			}
		}
	}
}
