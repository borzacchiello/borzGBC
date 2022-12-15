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

	InBootROM bool
	BootROM   []byte
}

func (cons *Console) Read(addr uint16) uint8 {
	switch {
	case 0x0000 <= addr && addr <= 0x3FFF:
		if cons.InBootROM && int(addr) < len(cons.BootROM) {
			return cons.BootROM[addr]
		}
		return cons.Cart.ROMBanks[0][addr]
	default:
		fmt.Printf("Unhandled read @ %04x\n", addr)
	}
	return 0
}

func (cons *Console) Write(addr uint16, value uint8) {
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
		CPU:       &z80cpu.Z80Cpu{},
		PPU:       MakePpu(videoDriver),
		BootROM:   boot,
		InBootROM: true,
	}
	res.CPU.Mem = res
	res.CPU.Reset()

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
