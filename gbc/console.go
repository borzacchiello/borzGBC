package gbc

import (
	"borzGBC/z80cpu"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

var InterruptVBlank z80cpu.Z80Interrupt = z80cpu.Z80Interrupt{
	Mask: 1, Addr: 0x40, Name: "VBLANK"}
var InterruptLCDStat z80cpu.Z80Interrupt = z80cpu.Z80Interrupt{
	Mask: 1 << 1, Addr: 0x48, Name: "LCDSTAT"}
var InterruptTimer z80cpu.Z80Interrupt = z80cpu.Z80Interrupt{
	Mask: 1 << 2, Addr: 0x50, Name: "TIMER"}
var InterruptSerial z80cpu.Z80Interrupt = z80cpu.Z80Interrupt{
	Mask: 1 << 3, Addr: 0x58, Name: "SERIAL"}
var InterruptJoypad z80cpu.Z80Interrupt = z80cpu.Z80Interrupt{
	Mask: 1 << 4, Addr: 0x60, Name: "JOYPAD"}

const GBCPU_FREQ = 4194304

type Frontend interface {
	NotifyAudioSample(l, r int8)
	SetPixel(x, y int, color uint32)
	CommitScreen()
}

type Console struct {
	Cart  *Cart
	CPU   *z80cpu.Z80Cpu
	PPU   *Ppu
	APU   *Apu
	DMA   *Dma
	timer *Timer
	Input *Joypad

	CGBMode bool
	CPUFreq int

	// Memory
	IOMem   [256]byte
	HighRAM [0x80]byte
	WorkRAM [8][0x1000]byte

	// CGB Registers and data
	RamBank         uint8 // RAM bank @ 0xD000-0xDFFF
	SpeedSwitch     uint8
	DoubleSpeedMode bool

	InBootROM bool
	BootROM   []byte

	// Debug Flags
	PrintDebug bool
	Verbose    bool
}

func (cons *Console) Save(encoder *gob.Encoder) {
	panicIfErr(encoder.Encode(cons.IOMem))
	panicIfErr(encoder.Encode(cons.HighRAM))
	panicIfErr(encoder.Encode(cons.WorkRAM))
	panicIfErr(encoder.Encode(cons.RamBank))
	panicIfErr(encoder.Encode(cons.SpeedSwitch))
	panicIfErr(encoder.Encode(cons.DoubleSpeedMode))
	panicIfErr(encoder.Encode(cons.InBootROM))
	panicIfErr(encoder.Encode(cons.BootROM))
	cons.Cart.Save(encoder)
	cons.CPU.Save(encoder)
	cons.PPU.Save(encoder)
	cons.APU.Save(encoder)
	cons.DMA.Save(encoder)
	cons.timer.Save(encoder)
	cons.Input.Save(encoder)
}

func (cons *Console) Load(decoder *gob.Decoder) {
	panicIfErr(decoder.Decode(&cons.IOMem))
	panicIfErr(decoder.Decode(&cons.HighRAM))
	panicIfErr(decoder.Decode(&cons.WorkRAM))
	panicIfErr(decoder.Decode(&cons.RamBank))
	panicIfErr(decoder.Decode(&cons.SpeedSwitch))
	panicIfErr(decoder.Decode(&cons.DoubleSpeedMode))
	panicIfErr(decoder.Decode(&cons.InBootROM))
	panicIfErr(decoder.Decode(&cons.BootROM))
	cons.Cart.Load(decoder)
	cons.CPU.Load(decoder)
	cons.PPU.Load(decoder)
	cons.APU.Load(decoder)
	cons.DMA.Load(decoder)
	cons.timer.Load(decoder)
	cons.Input.Load(decoder)
}

func (cons *Console) SaveState(n int) error {
	stateFilename := cons.Cart.filepath + fmt.Sprintf(".state.%d", n)
	f, err := os.Create(stateFilename)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := gob.NewEncoder(f)
	cons.Save(encoder)
	return nil
}

func (cons *Console) LoadState(n int) error {
	stateFilename := cons.Cart.filepath + fmt.Sprintf(".state.%d", n)
	f, err := os.Open(stateFilename)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := gob.NewDecoder(f)
	cons.Load(decoder)
	return nil
}

func (cons *Console) readIO(addr uint16) uint8 {
	switch {
	case addr == 0xFF00:
		return cons.Input.FrontState.PackButtons()
	case addr == 0xFF04:
		return uint8(cons.timer.DIV >> 8)
	case addr == 0xFF05:
		return cons.timer.TIMA
	case addr == 0xFF06:
		return cons.timer.TMA
	case addr == 0xFF07:
		return cons.timer.TAC
	case addr == 0xFF0F:
		return cons.CPU.IF
	case 0xFF10 <= addr && addr <= 0xFF14:
		// Audio - Channel 1: Tone & Sweep
		return cons.APU.Read(addr)
	case 0xFF16 <= addr && addr <= 0xFF19:
		// Audio - Channel 2: Tone
		return cons.APU.Read(addr)
	case 0xFF1A <= addr && addr <= 0xFF1F:
		// Audio - Channel 3: Wave Output
		return cons.APU.Read(addr)
	case 0xFF20 <= addr && addr <= 0xFF23:
		// Audio - Channel 4: Noise
		return cons.APU.Read(addr)
	case addr == 0xFF24:
		// Audio - Channel control/ON-OFF/Volume
		return cons.APU.Read(addr)
	case addr == 0xFF25:
		// Audio - Selection of sound output terminal
		return cons.APU.Read(addr)
	case addr == 0xFF26:
		// Audio - Sound on/off
		return cons.APU.Read(addr)
	case 0xFF30 <= addr && addr <= 0xFF3F:
		// Audio - Wave pattern RAM
		return cons.APU.Read(addr)
	case addr == 0xFF40:
		return cons.PPU.LCDC
	case addr == 0xFF41:
		return cons.PPU.STAT | 0x80
	case addr == 0xFF42:
		return cons.PPU.SCY
	case addr == 0xFF43:
		return cons.PPU.SCX
	case addr == 0xFF44:
		return cons.PPU.LY
	case addr == 0xFF45:
		return cons.PPU.LYC
	case addr == 0xFF46:
		return cons.DMA.GbDmaValue
	case addr == 0xFF47:
		return cons.PPU.BGP
	case addr == 0xFF48:
		return cons.PPU.OBP0
	case addr == 0xFF49:
		return cons.PPU.OBP1
	case addr == 0xFF4A:
		return cons.PPU.WY
	case addr == 0xFF4B:
		return cons.PPU.WX
	case addr == 0xFF4D:
		// CGB Only Register
		if !cons.CGBMode {
			return 0xFF
		}
		return cons.SpeedSwitch | 0x7E
	case addr == 0xFF4F:
		// CGB Only Register
		return cons.PPU.VRAMBank
	case addr == 0xFF50:
		if cons.InBootROM {
			return 0
		}
		return 1
	case addr == 0xFF55:
		// CGB Only Register
		return cons.DMA.HdmaControl
	case addr == 0xFF69:
		// CGB Only Register
		return cons.PPU.ReadCRamBg()
	case addr == 0xFF6B:
		// CGB Only Register
		return cons.PPU.ReadCRamObj()
	case addr == 0xFF6C:
		// CGB Only Register
		if cons.CGBMode {
			return 1
		}
		return 0
	case addr == 0xFF70:
		// CGB Only Register
		return cons.RamBank
	default:
		if cons.Verbose {
			fmt.Printf("Unhandled IO Read @ %04x\n", addr)
		}
	}
	return cons.IOMem[addr&0xFF]
}

func (cons *Console) dmaTransfer(value uint8) {
	addr := uint16(value) * 0x100

	for i := 0; i <= 0x9F; i++ {
		from := addr + uint16(i)
		to := uint16(0xFE00) + uint16(i)

		cons.Write(to, cons.Read(from))
	}
}

func (cons *Console) writeIO(addr uint16, value uint8) {
	cons.IOMem[addr&0xFF] = value
	switch {
	case addr == 0xFF00:
		cons.Input.FrontState.DirectionSelector = value&(1<<4) == 0
		cons.Input.FrontState.ActionSelector = value&(1<<5) == 0
	case addr == 0xFF04:
		cons.timer.reset()
	case addr == 0xFF05:
		cons.timer.TIMA = value
	case addr == 0xFF06:
		cons.timer.TMA = value
	case addr == 0xFF07:
		cons.timer.TAC = value
	case addr == 0xFF0F:
		cons.CPU.IF = value
	case 0xFF10 <= addr && addr <= 0xFF14:
		// Audio - Channel 1: Tone & Sweep
		cons.APU.Write(addr, value)
	case 0xFF16 <= addr && addr <= 0xFF19:
		// Audio - Channel 2: Tone
		cons.APU.Write(addr, value)
	case 0xFF1A <= addr && addr <= 0xFF1F:
		// Audio - Channel 3: Wave Output
		cons.APU.Write(addr, value)
	case 0xFF20 <= addr && addr <= 0xFF23:
		// Audio - Channel 4: Noise
		cons.APU.Write(addr, value)
	case addr == 0xFF24:
		// TODO: Audio - Channel control/ON-OFF/Volume
	case addr == 0xFF25:
		// Audio - Selection of sound output terminal
		cons.APU.Write(addr, value)
	case addr == 0xFF26:
		// Audio - Sound on/off
		cons.APU.Write(addr, value)
	case 0xFF30 <= addr && addr <= 0xFF3F:
		// Audio - Wave pattern RAM
		cons.APU.Write(addr, value)
	case addr == 0xFF40:
		cons.PPU.LCDC = value
	case addr == 0xFF41:
		cons.PPU.STAT = value
	case addr == 0xFF42:
		cons.PPU.SCY = value
	case addr == 0xFF43:
		cons.PPU.SCX = value
	case addr == 0xFF44:
		cons.PPU.LY = 0
	case addr == 0xFF45:
		cons.PPU.LYC = value
	case addr == 0xFF46:
		cons.DMA.GbDmaCycles = 648
		cons.DMA.GbDmaValue = value
	case addr == 0xFF47:
		cons.PPU.BGP = value
	case addr == 0xFF48:
		cons.PPU.OBP0 = value
	case addr == 0xFF49:
		cons.PPU.OBP1 = value
	case addr == 0xFF4A:
		cons.PPU.WY = value
	case addr == 0xFF4B:
		cons.PPU.WX = value
		if cons.PPU.WX < 7 {
			cons.PPU.WX = 7
		}
	case addr == 0xFF4D:
		// CGB Only Register
		cons.SpeedSwitch = (cons.SpeedSwitch & 0x80) | (value & 1)
	case addr == 0xFF4F:
		// CGB Only Register
		if cons.DMA.HdmaState == DMA_STATE_INACTIVE {
			cons.PPU.VRAMBank = value & 1
		}
	case addr == 0xFF50:
		if value != 0 {
			cons.InBootROM = false
		} else {
			cons.InBootROM = true
		}
	case addr == 0xFF51:
		// CGB Only Register
		cons.DMA.HdmaSrcHi = value
	case addr == 0xFF52:
		// CGB Only Register
		cons.DMA.HdmaSrcLo = value & 0xF0
	case addr == 0xFF53:
		// CGB Only Register
		cons.DMA.HdmaDstHi = value & 0x1F
	case addr == 0xFF54:
		// CGB Only Register
		cons.DMA.HdmaDstLo = value & 0xF0
	case addr == 0xFF55:
		// CGB Only Register
		cons.DMA.HdmaControl = value
		cons.DMA.HdmaWritten = true
	case addr == 0xFF68:
		// CGB Only Register
		cons.PPU.SetCRamBgAddr(value)
	case addr == 0xFF69:
		// CGB Only Register
		cons.PPU.WriteCRamBg(value)
	case addr == 0xFF6A:
		// CGB Only Register
		cons.PPU.SetCRAMObjAddr(value)
	case addr == 0xFF6B:
		// CGB Only Register
		cons.PPU.WriteCRamObj(value)
	case addr == 0xFF6C:
		// CGB Only Register
	case addr == 0xFF70:
		// CGB Only Register
		cons.RamBank = value & 7
		if cons.RamBank == 0 {
			cons.RamBank = 1
		}
	default:
		if cons.Verbose {
			fmt.Printf("Unhandled IO Write @ %04x <- %02x\n", addr, value)
		}
	}
}

func (cons *Console) Read(addr uint16) uint8 {
	switch {
	case addr <= 0x7FFF:
		if cons.InBootROM {
			if addr < 0x100 {
				return cons.BootROM[addr]
			}
			if 0x200 <= addr && addr <= 0x8FF && int(addr) < len(cons.BootROM) {
				return cons.BootROM[addr]
			}
		}
		return cons.Cart.Map.MapperRead(addr)
	case 0x8000 <= addr && addr <= 0x9FFF:
		return cons.PPU.ReadVRam(addr - 0x8000)
	case 0xA000 <= addr && addr <= 0xBFFF:
		return cons.Cart.Map.MapperRead(addr)
	case 0xC000 <= addr && addr <= 0xCFFF:
		return cons.WorkRAM[0][addr-0xC000]
	case 0xD000 <= addr && addr <= 0xDFFF:
		return cons.WorkRAM[cons.RamBank][addr-0xD000]
	case 0xE000 <= addr && addr <= 0xFDFF:
		return cons.Read(addr - 0x2000)
	case 0xFE00 <= addr && addr <= 0xFE9F:
		return cons.PPU.ReadOam(addr - 0xFE00)
	case 0xFEA0 <= addr && addr <= 0xFEFF:
		// Unusable memory
		return 0xFF
	case 0xFF00 <= addr && addr <= 0xFF7F:
		return cons.readIO(addr)
	case 0xFF80 <= addr && addr <= 0xFFFE:
		return cons.HighRAM[addr-0xFF80]
	case addr == 0xFFFF:
		return cons.CPU.IE
	default:
		if cons.Verbose {
			fmt.Printf("Unhandled read @ %04x\n", addr)
		}
	}
	return 0
}

func (cons *Console) Write(addr uint16, value uint8) {
	switch {
	case addr <= 0x7FFF:
		cons.Cart.Map.MapperWrite(addr, value)
		return
	case 0x8000 <= addr && addr <= 0x9FFF:
		cons.PPU.WriteVRam(addr-0x8000, value)
		return
	case 0xA000 <= addr && addr <= 0xBFFF:
		cons.Cart.Map.MapperWrite(addr, value)
		return
	case 0xC000 <= addr && addr <= 0xCFFF:
		cons.WorkRAM[0][addr-0xC000] = value
		return
	case 0xD000 <= addr && addr <= 0xDFFF:
		cons.WorkRAM[cons.RamBank][addr-0xD000] = value
		return
	case 0xE000 <= addr && addr <= 0xFDFF:
		cons.Write(addr-0x2000, value)
		return
	case 0xFE00 <= addr && addr <= 0xFE9F:
		cons.PPU.WriteOam(addr-0xFE00, value)
		return
	case 0xFEA0 <= addr && addr <= 0xFEFF:
		// Unusable memory
		return
	case 0xFF00 <= addr && addr <= 0xFF7F:
		cons.writeIO(addr, value)
		return
	case 0xFF80 <= addr && addr <= 0xFFFE:
		cons.HighRAM[addr-0xFF80] = value
		return
	case addr == 0xFFFF:
		cons.CPU.IE = value
		return
	}
	if cons.Verbose {
		fmt.Printf("Unhandled write @ %04x <- %02x\n", addr, value)
	}
}

func getExecutablePath() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", err
	}
	exPath := filepath.Dir(ex)
	return exPath, nil
}

func loadBoot(cart *Cart) ([]byte, error) {
	executableDir, err := getExecutablePath()
	if err != nil {
		return nil, err
	}

	bootromDir := "resources"
	if _, err := os.Stat("resources"); os.IsNotExist(err) {
		bootromDir = fmt.Sprintf("%s/resources", executableDir)
		if _, err := os.Stat(fmt.Sprintf("%s/resources", executableDir)); os.IsNotExist(err) {
			return nil, err
		}
	}

	if cart.header.CgbFlag != 0 {
		return os.ReadFile(fmt.Sprintf("%s/cgb.bin", bootromDir))
	}
	return os.ReadFile(fmt.Sprintf("%s/dmg.bin", bootromDir))
}

func loadSav(cart *Cart) error {
	savFilename := cart.filepath + ".sav"
	data, err := os.ReadFile(savFilename)
	if err != nil {
		// No save file
		return nil
	}

	if len(data) != len(cart.RAMBanks)*8192 {
		return CartError("Invalid SAV file")
	}

	for i := 0; i < len(cart.RAMBanks); i++ {
		off := i * 8192
		copy(cart.RAMBanks[i][:], data[off:off+8192])
	}
	return nil
}

func storeSav(cart *Cart) error {
	if len(cart.RAMBanks) == 0 {
		return nil
	}

	savFilename := cart.filepath + ".sav"
	f, err := os.Create(savFilename)
	if err != nil {
		return err
	}

	for i := 0; i < len(cart.RAMBanks); i++ {
		_, err = f.Write(cart.RAMBanks[i][:])
		if err != nil {
			return err
		}
	}
	if err = f.Close(); err != nil {
		return err
	}
	return nil
}

func MakeConsole(rom_filepath string, frontend Frontend) (*Console, error) {
	cart, err := LoadCartridge(rom_filepath)
	if err != nil {
		return nil, err
	}

	if err = loadSav(cart); err != nil {
		return nil, err
	}

	boot, err := loadBoot(cart)
	if err != nil {
		return nil, err
	}

	res := &Console{
		Cart:            cart,
		RamBank:         1,
		CGBMode:         cart.header.CgbFlag != 0,
		CPUFreq:         GBCPU_FREQ,
		BootROM:         boot,
		InBootROM:       true,
		SpeedSwitch:     0,
		DoubleSpeedMode: false,
		Verbose:         false,
	}
	res.DMA = MakeDma(res)
	res.PPU = MakePpu(res, frontend)
	res.APU = MakeApu(res, frontend)
	res.CPU = z80cpu.MakeZ80Cpu(res)
	res.Input = MakeJoypad(res)
	res.timer = MakeTimer(res)

	res.CPU.RegisterInterrupt(InterruptVBlank)
	res.CPU.RegisterInterrupt(InterruptLCDStat)
	res.CPU.RegisterInterrupt(InterruptTimer)
	res.CPU.RegisterInterrupt(InterruptSerial)
	res.CPU.RegisterInterrupt(InterruptJoypad)

	return res, nil
}

func (cons *Console) Destroy() error {
	if err := storeSav(cons.Cart); err != nil {
		return err
	}
	cons.Cart.Map.MapperClose()
	return nil
}

var prevTicks int = 0

func (cons *Console) Step() int {
	prevFrame := cons.PPU.FrameCount

	totTicks := 0
	for cons.PPU.FrameCount == prevFrame {

		if !cons.CPU.IsHalted {
			for cons.DMA.HdmaInProgress() {
				// The CPU is busy performing the HDMA
				cons.PPU.Tick(1)
				cons.APU.Tick(1)
				cons.DMA.Tick(1)
				cons.timer.Tick(1)
				totTicks += 1
			}
		}

		if cons.PrintDebug {
			var cpu *z80cpu.Z80Cpu = cons.CPU
			_, disas_str := cons.CPU.Disas.DisassembleOneFromCPU(cons.CPU)

			fmt.Fprintf(os.Stderr, "%s |CYC=%d PC=%04x SP=%04x A=%02x B=%02x C=%02x D=%02x E=%02x H=%02x L=%02x F=%02x IV=%02x PPUC=%04d LY=%02x LYC=%02x STAT=%02x LCDC=%02x SCX=%02x SCY=%02x WX=%02x WY=%02x MEM=%02x\n",
				disas_str, prevTicks, cpu.PC, cpu.SP, cpu.A, cpu.B, cpu.C, cpu.D, cpu.E, cpu.H, cpu.L, cpu.PackFlags(), cpu.IE&cpu.IF, cons.PPU.CycleCount, cons.PPU.LY, cons.PPU.LYC, cons.PPU.STAT, cons.PPU.LCDC, cons.PPU.SCX, cons.PPU.SCY, cons.PPU.WX, cons.PPU.WY, cons.Read(cpu.SP))
		}

		cpuTicks := cons.CPU.ExecOne()
		if cons.CPU.IsStopped {
			if cons.CGBMode && cons.SpeedSwitch&1 == 1 {
				cons.SpeedSwitch = (cons.SpeedSwitch ^ 0x80) & 0x80
				cons.DoubleSpeedMode = !cons.DoubleSpeedMode
				cons.CPU.IsStopped = false
				continue
			}
		}

		cons.PPU.Tick(cpuTicks)
		cons.APU.Tick(cpuTicks)
		cons.DMA.Tick(cpuTicks)
		cons.timer.Tick(cpuTicks)
		cons.Input.Tick(cpuTicks)

		totTicks += cpuTicks
		prevTicks = cpuTicks
	}

	return totTicks
}

func (cons *Console) GetMs(ticks int) int {
	res := ticks * 4 * 1000 / cons.CPUFreq
	if cons.DoubleSpeedMode {
		res /= 2
	}
	return res
}

func (cons *Console) GetBackgroundMapStr() string {
	out := "Current Background Map "
	if cons.PPU.BgTileMapDisplay() {
		out += "ONE @ 0x9C00:\n"
	} else {
		out += "ZERO @ 0x9800:\n"
	}
	base := TILE_MAP_ZERO_ADDRESS
	if cons.PPU.BgTileMapDisplay() {
		base = TILE_MAP_ONE_ADDRESS
	}
	for y := uint16(0); y < 32; y++ {
		out += fmt.Sprintf("  %02x: ", y)
		for x := uint16(0); x < 32; x++ {
			v1 := cons.PPU.VRAM[0][base+x+y*32-0x8000]
			v2 := cons.PPU.VRAM[1][base+x+y*32-0x8000]
			out += fmt.Sprintf("%02x:%02x ", v1, v2)
		}
		out += "\n"
	}
	return out
}
