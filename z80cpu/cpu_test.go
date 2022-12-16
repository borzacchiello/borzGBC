package z80cpu

import (
	"testing"
)

type TestMemory struct {
	mem [0xffff]uint8
}

func (mem *TestMemory) Read(addr uint16) uint8 {
	return mem.mem[addr]
}

func (mem *TestMemory) Write(addr uint16, val uint8) {
	mem.mem[addr] = val
}

func (mem *TestMemory) WriteBuffer(addr uint16, data []uint8) {
	for i, b := range data {
		mem.Write(addr+uint16(i), b)
	}
}

func TestLoadA(t *testing.T) {
	var prog = []byte{
		0x3e, 0x0a, // ld  a, 10
		0x06, 0x14, // ld  b, 20
		0x80, //       add a, b
	}
	memory := &TestMemory{}
	memory.WriteBuffer(0, prog)

	cpu := Z80Cpu{Mem: memory}
	cpu.ExecOne()
	cpu.ExecOne()
	cpu.ExecOne()

	if cpu.a != 30 || cpu.b != 20 {
		t.Errorf("cpu.a=%d (exp: 30); cpu.b=%d (exp: 20)", cpu.a, cpu.b)
	}
}

func TestXor(t *testing.T) {
	var prog = []byte{
		0x3e, 0x10, // ld a, 0x10
		0x06, 0x20, // ld b, 0x20
		0xa8, //       xor b
	}

	memory := &TestMemory{}
	memory.WriteBuffer(0, prog)

	cpu := Z80Cpu{Mem: memory}
	cpu.ExecOne()
	cpu.ExecOne()
	cpu.ExecOne()

	if cpu.a != 0x30 {
		t.Errorf("cpu.a=%02x (exp: 0x30)", cpu.a)
	}
}

func TestJRNZ(t *testing.T) {
	var prog = []byte{
		0xcb, 0x47, //     bit 0, a
		0x20, 0x02, //     jr nz, T1
		0x06, 0x00, //     ld b, 0
		0x06, 0x01} // T1: ld b, 1

	memory := &TestMemory{}
	memory.WriteBuffer(0, prog)

	cpu := Z80Cpu{Mem: memory}
	cpu.a = 0
	cpu.ExecOne()
	cpu.ExecOne()
	cpu.ExecOne()

	if cpu.b != 0 {
		t.Errorf("cpu.b=%d (exp: 0)", cpu.b)
	}

	cpu.Reset()
	cpu.a = 1
	cpu.ExecOne()
	cpu.ExecOne()
	cpu.ExecOne()

	if cpu.b != 1 {
		t.Errorf("cpu.b=%d (exp: 1)", cpu.b)
	}
}

func TestBit(t *testing.T) {
	var prog = []byte{
		0x3e, 0x10, // ld a, 0x10
		0xcb, 0x67, // bit 4, a
		0xcb, 0x47, // bit 0, a
	}

	memory := &TestMemory{}
	memory.WriteBuffer(0, prog)

	cpu := Z80Cpu{Mem: memory}
	cpu.ExecOne()
	cpu.ExecOne()
	if cpu.flagWasZero {
		t.Errorf("flagWasZero should be false")
	}

	cpu.ExecOne()
	if !cpu.flagWasZero {
		t.Errorf("flagWasZero should be true")
	}
}

func TestRegcoupleInc(t *testing.T) {
	var prog = []byte{
		0x03, // inc bc
		0x03, // inc bc
		0x03, // inc bc
		0x03, // inc bc
	}
	memory := &TestMemory{}
	memory.WriteBuffer(0, prog)

	cpu := Z80Cpu{Mem: memory}
	cpu.ExecOne()
	cpu.ExecOne()
	cpu.ExecOne()

	if cpu.b != 0 || cpu.c != 3 {
		t.Errorf("cpu.b=%d (exp: 0); cpu.c=%d (exp: 3)", cpu.b, cpu.c)
	}

	cpu.b = 1
	cpu.c = 255
	cpu.ExecOne()

	if cpu.b != 2 || cpu.c != 0 {
		t.Errorf("cpu.b=%d (exp: 2); cpu.c=%d (exp: 0)", cpu.b, cpu.c)
	}
}

func TestProgHLToHex(t *testing.T) {
	var prog = []byte{
		0x00, 0x00, 0x00, // 00: nop (x3)
		0x00,             // 03: nop
		0xcd, 0x08, 0x00, // 04: call 0x08
		0x76,       //       07: halt
		0x3e, 0x30, //       08: ld a, 0x30
		0xd3, 0x01, //       0a: out (0x01), a
		0x3e, 0x78, //       0c: ld a, 0x78
		0xd3, 0x01, //       0e: out (0x01), a
		0x4c,             // 10: ld c, h
		0xcd, 0x19, 0x00, // 11: call 0x19
		0x4d,             // 14: ld c, l
		0xcd, 0x19, 0x00, // 15: call 0x19
		0xc9,             // 18: ret
		0x79,             // 19: ld a, c
		0x1f,             // 1a: rra
		0x1f,             // 1b: rra
		0x1f,             // 1c: rra
		0x1f,             // 1d: rra
		0xcd, 0x22, 0x00, // 1e: call 0x22
		0x79,       //       21: ld a, c
		0xe6, 0x0f, //       22: and 0x0f
		0xc6, 0x90, //       24: add a, 0x90
		0x27,       //       26: daa
		0xce, 0x40, //       27: adc a,0x40
		0x27,       //       29: daa
		0xd3, 0x01, //       2a: out (0x01), a
		0xc9} //             2c: ret

	memory := &TestMemory{}
	memory.WriteBuffer(0, prog)

	cpu := MakeZ80Cpu(memory)

	runProg := func(inp uint16) string {
		cpu.Reset()

		cpu.h = uint8(inp >> 8)
		cpu.l = uint8(inp & 0xff)

		for !cpu.isHalted {
			cpu.ExecOne()
		}
		return string(cpu.OutBuffer)
	}

	out := runProg(0xdead)
	if out != "0xDEAD" {
		t.Errorf("output=%s, expected 0xDEAD", out)
	}

	out = runProg(0xbeef)
	if out != "0xBEEF" {
		t.Errorf("output=%s, expected 0xBEEF", out)
	}
}
