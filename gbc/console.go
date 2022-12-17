package gbc

import (
	"borzGBC/z80cpu"
	"fmt"
	"io/ioutil"
)

var InterruptVBlank z80cpu.Z80Interrupt = z80cpu.Z80Interrupt{
	Mask: 0, Addr: 0x40, Name: "VBLANK"}
var InterruptLCDStat z80cpu.Z80Interrupt = z80cpu.Z80Interrupt{
	Mask: 1, Addr: 0x48, Name: "LCDSTAT"}
var InterruptTimer z80cpu.Z80Interrupt = z80cpu.Z80Interrupt{
	Mask: 2, Addr: 0x50, Name: "TIMER"}
var InterruptSerial z80cpu.Z80Interrupt = z80cpu.Z80Interrupt{
	Mask: 3, Addr: 0x58, Name: "SERIAL"}
var InterruptJoypad z80cpu.Z80Interrupt = z80cpu.Z80Interrupt{
	Mask: 4, Addr: 0x60, Name: "JOYPAD"}

type Console struct {
	Cart *Cart
	CPU  *z80cpu.Z80Cpu
	PPU  *Ppu

	// Memory
	HighRAM [0x80]byte
	WorkRAM [0x8000]byte
	OamRAM  [0xA0]byte

	InBootROM bool
	BootROM   []byte
}

func (cons *Console) readIO(addr uint16) uint8 {
	switch {
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
			return 1
		}
		return 0
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
		if value == 0 {
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
		fmt.Printf("Unhandled IO Read @ %04x\n", addr)
	}
}

func (cons *Console) Read(addr uint16) uint8 {
	switch {
	case 0x0000 <= addr && addr <= 0x3FFF:
		if cons.InBootROM && int(addr) < len(cons.BootROM) {
			return cons.BootROM[addr]
		}
		return cons.Cart.ROMBanks[0][addr]
	case 0x8000 <= addr && addr <= 0x9FFF:
		return cons.PPU.Read(addr - 0x8000)
	case 0xC000 <= addr && addr <= 0xDFFF:
		return cons.WorkRAM[addr-0xC000]
	case 0xE000 <= addr && addr <= 0xFDFF:
		return cons.Read(addr - 0x2000)
	case 0xFE00 <= addr && addr <= 0xFE9F:
		return cons.OamRAM[addr-0xFE00]
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
	case 0x8000 <= addr && addr <= 0x9FFF:
		cons.PPU.Write(addr-0x8000, value)
		return
	case 0xC000 <= addr && addr <= 0xDFFF:
		cons.WorkRAM[addr-0x8000] = value
		return
	case 0xE000 <= addr && addr <= 0xFDFF:
		cons.Write(addr-0x2000, value)
		return
	case 0xFE00 <= addr && addr <= 0xFE9F:
		cons.OamRAM[addr-0xFE00] = value
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
	return ioutil.ReadFile("BootROMs/dmg.bin")
}

func MakeConsole(rom_filepath string, videoDriver VideoDriver) (*Console, error) {
	cart, err := LoadCartridge(rom_filepath)
	if err != nil {
		return nil, err
	}

	boot, err := loadBoot(cart)
	if err != nil {
		return nil, err
	}

	res := &Console{
		Cart:      cart,
		BootROM:   boot,
		InBootROM: true,
	}
	res.PPU = MakePpu(res, videoDriver)
	res.CPU = z80cpu.MakeZ80Cpu(res)

	res.CPU.RegisterInterrupt(InterruptVBlank)
	res.CPU.RegisterInterrupt(InterruptLCDStat)
	res.CPU.RegisterInterrupt(InterruptTimer)
	res.CPU.RegisterInterrupt(InterruptSerial)
	res.CPU.RegisterInterrupt(InterruptJoypad)

	return res, nil
}

func (cons *Console) Run() {
	for {
		cpuCycles := cons.CPU.ExecOne()
		cons.PPU.Tick(cpuCycles)
	}
}
