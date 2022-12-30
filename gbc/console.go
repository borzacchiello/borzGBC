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

	CGBMode bool
	CPUFreq int

	// Memory
	HighRAM [0x80]byte
	WorkRAM [8][0x1000]byte

	// CGB Registers and data
	RamBank     uint8 // RAM bank @ 0xD000-0xDFFF
	DmaSrc      uint16
	DmaDst      uint16
	SpeedSwitch uint8

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
		return 0
	case addr == 0xFF25:
		// TODO: Audio - Selection of sound output terminal
		return 0xFF
	case addr == 0xFF26:
		// TODO: Audio - Sound on/off
		return 0
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
	case addr == 0xFF4A:
		return cons.PPU.WY
	case addr == 0xFF4B:
		return cons.PPU.WX
	case addr == 0xFF4D:
		// CGB Only Register
		// FIXME: Implement speed switch
		return cons.SpeedSwitch
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
		// FIXME: This should indicate whether the DMA transfer is happening.
		//        Currently we do the transfer in one shot, so it returns always 0xFF
		return 0xFF
	case addr == 0xFF69:
		// CGB Only Register
		return cons.PPU.ReadCRamBg()
	case addr == 0xFF6B:
		// CGB Only Register
		return cons.PPU.ReadCRamObj()
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

func (cons *Console) cgbDmaTransfer(value uint8) {
	// FIXME: The transfer does not happen in one shot, but depending on the must
	//        significant bit of "value" should be performed in different ways
	src := cons.DmaSrc & 0xFFF0
	dst := cons.DmaDst&0x1FF0 | 0x8000
	len := (uint16(value&0x7F) + 1) * 16

	for i := uint16(0); i < len; i++ {
		cons.Write(dst+i, cons.Read(src+i))
	}
}

func (cons *Console) writeIO(addr uint16, value uint8) {
	switch {
	case addr == 0xFF00:
		cons.Input.DirectionSelector = value&(1<<4) == 0
		cons.Input.ActionSelector = value&(1<<5) == 0
	case addr == 0xFF04:
		cons.timer.DIV = 0
		return
	case addr == 0xFF05:
		cons.timer.TIMA = value
		return
	case addr == 0xFF06:
		cons.timer.TMA = value
		return
	case addr == 0xFF07:
		cons.timer.TAC = value
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
	case addr == 0xFF4A:
		cons.PPU.WY = value
		return
	case addr == 0xFF4B:
		cons.PPU.WX = value
		return
	case addr == 0xFF4D:
		// CGB Only Register
		cons.SpeedSwitch = value&1 | 0x7E
		return
	case addr == 0xFF4F:
		// CGB Only Register
		if value == 0 {
			cons.PPU.VRAMBank = 0
		} else {
			cons.PPU.VRAMBank = 1
		}
		return
	case addr == 0xFF50:
		if value != 0 {
			cons.InBootROM = false
		} else {
			cons.InBootROM = true
		}
		return
	case addr == 0xFF51:
		// CGB Only Register
		cons.DmaSrc |= uint16(value) << 8
		return
	case addr == 0xFF52:
		// CGB Only Register
		cons.DmaSrc |= uint16(value)
		return
	case addr == 0xFF53:
		// CGB Only Register
		cons.DmaDst |= uint16(value) << 8
		return
	case addr == 0xFF54:
		// CGB Only Register
		cons.DmaDst |= uint16(value)
		return
	case addr == 0xFF55:
		// CGB Only Register
		cons.cgbDmaTransfer(value)
		return
	case addr == 0xFF68:
		// CGB Only Register
		cons.PPU.SetCRamBgAddr(value)
		return
	case addr == 0xFF69:
		// CGB Only Register
		cons.PPU.WriteCRamBg(value)
		return
	case addr == 0xFF6A:
		// CGB Only Register
		cons.PPU.SetCRAMObjAddr(value)
		return
	case addr == 0xFF6B:
		// CGB Only Register
		cons.PPU.WriteCRamObj(value)
		return
	case addr == 0xFF70:
		// CGB Only Register
		cons.RamBank = value & 7
		if cons.RamBank == 0 {
			cons.RamBank = 1
		}
		return
	default:
		fmt.Printf("Unhandled IO Write @ %04x <- %02x\n", addr, value)
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
		fmt.Printf("Unhandled read @ %04x\n", addr)
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
	fmt.Printf("Unhandled write @ %04x <- %02x\n", addr, value)
}

func loadBoot(cart *Cart) ([]byte, error) {
	// FIXME: load the correct ROM and parametrize BootROMs location
	if cart.header.CgbFlag != 0 {
		return os.ReadFile("BootROMs/cgb.bin")
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
		RamBank:   1,
		CGBMode:   cart.header.CgbFlag != 0,
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

var prevTicks int = 0

func (cons *Console) Step() int {
	prevFrame := cons.PPU.FrameCount

	totTicks := 0
	for cons.PPU.FrameCount == prevFrame {

		if cons.PrintDebug {
			var cpu *z80cpu.Z80Cpu = cons.CPU
			_, disas_str := cons.CPU.Disas.DisassembleOneFromCPU(cons.CPU)

			fmt.Fprintf(os.Stderr, "%s |CYC=%d PC=%04x SP=%04x A=%02x B=%02x C=%02x D=%02x E=%02x H=%02x L=%02x F=%02x IV=%02x PPUC=%04d LY=%02x LYC=%02x STAT=%02x LCDC=%02x SCX=%02x SCY=%02x WX=%02x WY=%02x MEM=%02x\n",
				disas_str, prevTicks, cpu.PC, cpu.SP, cpu.A, cpu.B, cpu.C, cpu.D, cpu.E, cpu.H, cpu.L, cpu.PackFlags(), cpu.IE&cpu.IF, cons.PPU.CycleCount, cons.PPU.LY, cons.PPU.LYC, cons.PPU.STAT, cons.PPU.LCDC, cons.PPU.SCX, cons.PPU.SCY, cons.PPU.WX, cons.PPU.WY, cons.Read(cpu.SP))
		}

		cpuTicks := cons.CPU.ExecOne()
		cons.timer.Tick(cpuTicks)
		cons.PPU.Tick(cpuTicks)

		totTicks += cpuTicks
		prevTicks = cpuTicks
	}

	return totTicks
}

func (cons *Console) GetMs(ticks int) int {
	return ticks * 4 * 1000 / cons.CPUFreq
}
