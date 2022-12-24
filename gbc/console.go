package gbc

import (
	"borzGBC/z80cpu"
	"fmt"
	"os"
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

type Console struct {
	Cart  *Cart
	CPU   *z80cpu.Z80Cpu
	PPU   *Ppu
	timer *Timer
	Input *Joypad

	CPUFreq int

	// Memory
	HighRAM [0x80]byte
	WorkRAM [0x8000]byte

	InBootROM bool
	BootROM   []byte

	// Debug Flags
	PrintDebug bool
}

func (cons *Console) readIO(addr uint16) uint8 {
	switch {
	case addr == 0xFF00:
		return cons.Input.PackButtons()
	case addr == 0xFF04:
		return cons.timer.DIV
	case addr == 0xFF0F:
		return cons.CPU.IF
	case 0xFF10 <= addr && addr <= 0xFF14:
		// TODO: Audio - Channel 1: Tone & Sweep
		return 0xFF
	case 0xFF16 <= addr && addr <= 0xFF19:
		// TODO: Audio - Channel 2: Tone
		return 0xFF
	case 0xFF1A <= addr && addr <= 0xFF1F:
		// TODO: Audio - Channel 3: Wave Output
		return 0xFF
	case 0xFF20 <= addr && addr <= 0xFF23:
		// TODO: Audio - Channel 4: Noise
		return 0xFF
	case addr == 0xFF24:
		// TODO: Audio - Channel control/ON-OFF/Volume
		return 0xFF
	case addr == 0xFF25:
		// TODO: Audio - Selection of sound output terminal
		return 0xFF
	case addr == 0xFF26:
		// TODO: Audio - Sound on/off
		return 0xFF
	case 0xFF30 <= addr && addr <= 0xFF3F:
		// TODO: Audio - Wave pattern RAM
		return 0xFF
	case addr == 0xFF40:
		return cons.PPU.LCDC
	case addr == 0xFF41:
		return cons.PPU.STAT
	case addr == 0xFF42:
		return cons.PPU.SCY
	case addr == 0xFF43:
		return cons.PPU.SCX
	case addr == 0xFF44:
		return cons.PPU.LY
	case addr == 0xFF45:
		return cons.PPU.LYC
	case addr == 0xFF47:
		return cons.PPU.BGP
	case addr == 0xFF48:
		return cons.PPU.OBP0
	case addr == 0xFF49:
		return cons.PPU.OBP1
	case addr == 0xFF50:
		if cons.InBootROM {
			return 0
		}
		return 1
	case addr == 0xFF4A:
		return cons.PPU.WY
	case addr == 0xFF4B:
		return cons.PPU.WX
	default:
		fmt.Printf("Unhandled IO Read @ %04x\n", addr)
	}
	return 0xFF
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
	switch {
	case addr == 0xFF00:
		cons.Input.DirectionSelector = value&(1<<4) == 0
		cons.Input.ActionSelector = value&(1<<5) == 0
	case addr == 0xFF04:
		cons.timer.Reset()
		return
	case addr == 0xFF0F:
		cons.CPU.IF = value
		return
	case 0xFF10 <= addr && addr <= 0xFF14:
		// TODO: Audio - Channel 1: Tone & Sweep
		return
	case 0xFF16 <= addr && addr <= 0xFF19:
		// TODO: Audio - Channel 2: Tone
		return
	case 0xFF1A <= addr && addr <= 0xFF1F:
		// TODO: Audio - Channel 3: Wave Output
		return
	case 0xFF20 <= addr && addr <= 0xFF23:
		// TODO: Audio - Channel 4: Noise
		return
	case addr == 0xFF24:
		// TODO: Audio - Channel control/ON-OFF/Volume
		return
	case addr == 0xFF25:
		// TODO: Audio - Selection of sound output terminal
		return
	case addr == 0xFF26:
		// TODO: Audio - Sound on/off
		return
	case 0xFF30 <= addr && addr <= 0xFF3F:
		// TODO: Audio - Wave pattern RAM
		return
	case addr == 0xFF40:
		cons.PPU.LCDC = value
		return
	case addr == 0xFF41:
		cons.PPU.STAT = value
		return
	case addr == 0xFF42:
		cons.PPU.SCY = value
		return
	case addr == 0xFF43:
		cons.PPU.SCX = value
		return
	case addr == 0xFF44:
		cons.PPU.LY = 0
		return
	case addr == 0xFF45:
		cons.PPU.LYC = value
		return
	case addr == 0xFF46:
		cons.dmaTransfer(value)
		return
	case addr == 0xFF47:
		cons.PPU.BGP = value
		return
	case addr == 0xFF48:
		cons.PPU.OBP0 = value
		return
	case addr == 0xFF49:
		cons.PPU.OBP1 = value
		return
	case addr == 0xFF50:
		if value == 1 {
			cons.InBootROM = false
		} else {
			cons.InBootROM = true
		}
		return
	case addr == 0xFF4A:
		cons.PPU.WY = value
		return
	case addr == 0xFF4B:
		cons.PPU.WX = value
		return
	default:
		fmt.Printf("Unhandled IO Write @ %04x <- %02x\n", addr, value)
	}
}

func (cons *Console) Read(addr uint16) uint8 {
	switch {
	case 0x0000 <= addr && addr <= 0x7FFF:
		if cons.InBootROM && int(addr) < len(cons.BootROM) {
			return cons.BootROM[addr]
		}
		return cons.Cart.Map.MapperRead(addr)
	case 0x8000 <= addr && addr <= 0x9FFF:
		return cons.PPU.ReadVRam(addr - 0x8000)
	case 0xA000 <= addr && addr <= 0xBFFF:
		return cons.Cart.Map.MapperRead(addr)
	case 0xC000 <= addr && addr <= 0xDFFF:
		return cons.WorkRAM[addr-0xC000]
	case 0xE000 <= addr && addr <= 0xFDFF:
		return cons.Read(addr - 0x2000)
	case 0xFE00 <= addr && addr <= 0xFE9F:
		return cons.PPU.ReadOam(addr - 0xFE00)
	case 0xFF00 <= addr && addr <= 0xFF7F:
		return cons.readIO(addr)
	case 0xFF80 <= addr && addr <= 0xFFFE:
		return cons.HighRAM[addr-0xFF80]
	case addr == 0xFFFF:
		return cons.CPU.IE
	default:
		fmt.Printf("Unhandled read @ %04x\n", addr)
	}
	return 0
}

func (cons *Console) Write(addr uint16, value uint8) {
	switch {
	case 0x0000 <= addr && addr <= 0x7FFF:
		cons.Cart.Map.MapperWrite(addr, value)
		return
	case 0x8000 <= addr && addr <= 0x9FFF:
		cons.PPU.WriteVRam(addr-0x8000, value)
		return
	case 0xA000 <= addr && addr <= 0xBFFF:
		cons.Cart.Map.MapperWrite(addr, value)
		return
	case 0xC000 <= addr && addr <= 0xDFFF:
		cons.WorkRAM[addr-0xC000] = value
		return
	case 0xE000 <= addr && addr <= 0xFDFF:
		cons.Write(addr-0x2000, value)
		return
	case 0xFE00 <= addr && addr <= 0xFE9F:
		cons.PPU.WriteOam(addr-0xFE00, value)
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
	fmt.Printf("Unhandled write @ %04x <- %02x\n", addr, value)
}

func loadBoot(cart *Cart) ([]byte, error) {
	// FIXME: load the correct ROM and parametrize BootROMs location
	if cart.header.CgbFlag == 0xC0 {
		return nil, CartError("Unsupported cartridge type")
	}
	return os.ReadFile("BootROMs/dmg.bin")
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

func MakeConsole(rom_filepath string, videoDriver VideoDriver) (*Console, error) {
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
		Cart:      cart,
		Input:     &Joypad{},
		CPUFreq:   GBCPU_FREQ,
		BootROM:   boot,
		InBootROM: true,
	}
	res.PPU = MakePpu(res, videoDriver)
	res.CPU = z80cpu.MakeZ80Cpu(res)
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
	return nil
}

var prevCycles int = 0

func (cons *Console) Step() int {
	prevFrame := cons.PPU.FrameCount

	totCycles := 0
	for cons.PPU.FrameCount == prevFrame {

		if cons.PrintDebug {
			var cpu *z80cpu.Z80Cpu = cons.CPU
			_, disas_str := cons.CPU.Disas.DisassembleOneFromCPU(cons.CPU)

			fmt.Fprintf(os.Stderr, "%s |CYC=%d PC=%04x SP=%04x A=%02x B=%02x C=%02x D=%02x E=%02x H=%02x L=%02x F=%02x IV=%02x PPUC=%04d LY=%02x LYC=%02x STAT=%02x LCDC=%02x SCX=%02x SCY=%02x WX=%02x WY=%02x MEM=%02x\n",
				disas_str, prevCycles, cpu.PC, cpu.SP, cpu.A, cpu.B, cpu.C, cpu.D, cpu.E, cpu.H, cpu.L, cpu.PackFlags(), cpu.IE&cpu.IF, cons.PPU.CycleCount, cons.PPU.LY, cons.PPU.LYC, cons.PPU.STAT, cons.PPU.LCDC, cons.PPU.SCX, cons.PPU.SCY, cons.PPU.WX, cons.PPU.WY, cons.Read(cpu.SP))
		}

		cpuCycles := cons.CPU.ExecOne()
		cons.PPU.Tick(cpuCycles)
		cons.timer.Tick(cpuCycles)

		totCycles += cpuCycles
		prevCycles = cpuCycles
	}

	return totCycles
}

func (cons *Console) GetMs(cycles int) int {
	return cycles * 1000 / cons.CPUFreq
}
