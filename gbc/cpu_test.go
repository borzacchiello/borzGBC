package gbc

type TestMemory struct {
	mem [0xffff]uint8
}

func (mem *TestMemory) Read(addr uint16) uint8 {
	return mem.mem[addr]
}

func (mem *TestMemory) Write(addr uint16, val uint8) {
	mem.mem[addr] = val
}
