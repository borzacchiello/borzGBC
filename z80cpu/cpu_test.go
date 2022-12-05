package z80cpu

import "testing"

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
