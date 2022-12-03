package gbc

type Memory interface {
	Read(uint16) uint8
	Write(uint16, uint8)
}

type GBCpu struct {
	memory              Memory
	a, b, c, d, e, h, l uint8

	flagWasZero, flagWasSub, flagHalfCarry, flagCarry bool

	sp, pc uint16
}

func MakeGBCpu(mem Memory) *GBCpu {
	return &GBCpu{
		memory: mem,
	}
}

func (cpu *GBCpu) PackFlags() uint8 {
	res := 0
	if cpu.flagWasZero {
		res |= 0x80
	}
	if cpu.flagWasSub {
		res |= 0x40
	}
	if cpu.flagHalfCarry {
		res |= 0x20
	}
	if cpu.flagCarry {
		res |= 0x10
	}
	return uint8(res)
}

func (cpu *GBCpu) UnpackFlags(f uint8) {
	if f&0x80 != 0 {
		cpu.flagWasZero = true
	}
	if f&0x40 != 0 {
		cpu.flagWasSub = true
	}
	if f&0x20 != 0 {
		cpu.flagHalfCarry = true
	}
	if f&0x10 != 0 {
		cpu.flagCarry = true
	}
}

func (cpu *GBCpu) getPC8() uint8 {
	v := cpu.memory.Read(cpu.pc)

	cpu.pc += 1
	return v
}

func (cpu *GBCpu) getPC16() uint16 {
	l := cpu.memory.Read(cpu.pc)
	h := cpu.memory.Read(cpu.pc + 1)

	cpu.pc += 2
	return (uint16(h) << 8) | uint16(l)
}

func handler_nop() {

}

func handler_ld(dst1, dst2 *uint8, src uint16) {
	*dst1 = uint8((src >> 0) & uint16(0xFF))
	*dst2 = uint8((src >> 8) & uint16(0xFF))
}

func handler_add_a(cpu *GBCpu, val uint8) {
	old_a := cpu.a
	res := uint16(old_a) + uint16(val)
	cpu.a = uint8(res & 0xff)

	cpu.flagWasZero = cpu.a == 0
	cpu.flagWasSub = false
	cpu.flagHalfCarry = (old_a&0xf)+(val&0xf) > 0xf
	cpu.flagCarry = res&0x100 != 0
}

var handlers = [256]func(*GBCpu){
	func(cpu *GBCpu) { handler_nop() },                             // 00
	func(cpu *GBCpu) { handler_ld(&cpu.b, &cpu.a, cpu.getPC16()) }, // 01
	func(cpu *GBCpu) { panic("Opcode 02 unimplemented") },          // 02
	func(cpu *GBCpu) { panic("Opcode 03 unimplemented") },          // 03
	func(cpu *GBCpu) { panic("Opcode 04 unimplemented") },          // 04
	func(cpu *GBCpu) { panic("Opcode 05 unimplemented") },          // 05
	func(cpu *GBCpu) { panic("Opcode 06 unimplemented") },          // 06
	func(cpu *GBCpu) { panic("Opcode 07 unimplemented") },          // 07
	func(cpu *GBCpu) { panic("Opcode 08 unimplemented") },          // 08
	func(cpu *GBCpu) { panic("Opcode 09 unimplemented") },          // 09
	func(cpu *GBCpu) { panic("Opcode 0A unimplemented") },          // 0A
	func(cpu *GBCpu) { panic("Opcode 0B unimplemented") },          // 0B
	func(cpu *GBCpu) { panic("Opcode 0C unimplemented") },          // 0C
	func(cpu *GBCpu) { panic("Opcode 0D unimplemented") },          // 0D
	func(cpu *GBCpu) { panic("Opcode 0E unimplemented") },          // 0E
	func(cpu *GBCpu) { panic("Opcode 0F unimplemented") },          // 0F
	func(cpu *GBCpu) { panic("Opcode 10 unimplemented") },          // 10
	func(cpu *GBCpu) { panic("Opcode 11 unimplemented") },          // 11
	func(cpu *GBCpu) { panic("Opcode 12 unimplemented") },          // 12
	func(cpu *GBCpu) { panic("Opcode 13 unimplemented") },          // 13
	func(cpu *GBCpu) { panic("Opcode 14 unimplemented") },          // 14
	func(cpu *GBCpu) { panic("Opcode 15 unimplemented") },          // 15
	func(cpu *GBCpu) { panic("Opcode 16 unimplemented") },          // 16
	func(cpu *GBCpu) { panic("Opcode 17 unimplemented") },          // 17
	func(cpu *GBCpu) { panic("Opcode 18 unimplemented") },          // 18
	func(cpu *GBCpu) { panic("Opcode 19 unimplemented") },          // 19
	func(cpu *GBCpu) { panic("Opcode 1A unimplemented") },          // 1A
	func(cpu *GBCpu) { panic("Opcode 1B unimplemented") },          // 1B
	func(cpu *GBCpu) { panic("Opcode 1C unimplemented") },          // 1C
	func(cpu *GBCpu) { panic("Opcode 1D unimplemented") },          // 1D
	func(cpu *GBCpu) { panic("Opcode 1E unimplemented") },          // 1E
	func(cpu *GBCpu) { panic("Opcode 1F unimplemented") },          // 1F
	func(cpu *GBCpu) { panic("Opcode 20 unimplemented") },          // 20
	func(cpu *GBCpu) { panic("Opcode 21 unimplemented") },          // 21
	func(cpu *GBCpu) { panic("Opcode 22 unimplemented") },          // 22
	func(cpu *GBCpu) { panic("Opcode 23 unimplemented") },          // 23
	func(cpu *GBCpu) { panic("Opcode 24 unimplemented") },          // 24
	func(cpu *GBCpu) { panic("Opcode 25 unimplemented") },          // 25
	func(cpu *GBCpu) { panic("Opcode 26 unimplemented") },          // 26
	func(cpu *GBCpu) { panic("Opcode 27 unimplemented") },          // 27
	func(cpu *GBCpu) { panic("Opcode 28 unimplemented") },          // 28
	func(cpu *GBCpu) { panic("Opcode 29 unimplemented") },          // 29
	func(cpu *GBCpu) { panic("Opcode 2A unimplemented") },          // 2A
	func(cpu *GBCpu) { panic("Opcode 2B unimplemented") },          // 2B
	func(cpu *GBCpu) { panic("Opcode 2C unimplemented") },          // 2C
	func(cpu *GBCpu) { panic("Opcode 2D unimplemented") },          // 2D
	func(cpu *GBCpu) { panic("Opcode 2E unimplemented") },          // 2E
	func(cpu *GBCpu) { panic("Opcode 2F unimplemented") },          // 2F
	func(cpu *GBCpu) { panic("Opcode 30 unimplemented") },          // 30
	func(cpu *GBCpu) { panic("Opcode 31 unimplemented") },          // 31
	func(cpu *GBCpu) { panic("Opcode 32 unimplemented") },          // 32
	func(cpu *GBCpu) { panic("Opcode 33 unimplemented") },          // 33
	func(cpu *GBCpu) { panic("Opcode 34 unimplemented") },          // 34
	func(cpu *GBCpu) { panic("Opcode 35 unimplemented") },          // 35
	func(cpu *GBCpu) { panic("Opcode 36 unimplemented") },          // 36
	func(cpu *GBCpu) { panic("Opcode 37 unimplemented") },          // 37
	func(cpu *GBCpu) { panic("Opcode 38 unimplemented") },          // 38
	func(cpu *GBCpu) { panic("Opcode 39 unimplemented") },          // 39
	func(cpu *GBCpu) { panic("Opcode 3A unimplemented") },          // 3A
	func(cpu *GBCpu) { panic("Opcode 3B unimplemented") },          // 3B
	func(cpu *GBCpu) { panic("Opcode 3C unimplemented") },          // 3C
	func(cpu *GBCpu) { panic("Opcode 3D unimplemented") },          // 3D
	func(cpu *GBCpu) { panic("Opcode 3E unimplemented") },          // 3E
	func(cpu *GBCpu) { panic("Opcode 3F unimplemented") },          // 3F
	func(cpu *GBCpu) { panic("Opcode 40 unimplemented") },          // 40
	func(cpu *GBCpu) { panic("Opcode 41 unimplemented") },          // 41
	func(cpu *GBCpu) { panic("Opcode 42 unimplemented") },          // 42
	func(cpu *GBCpu) { panic("Opcode 43 unimplemented") },          // 43
	func(cpu *GBCpu) { panic("Opcode 44 unimplemented") },          // 44
	func(cpu *GBCpu) { panic("Opcode 45 unimplemented") },          // 45
	func(cpu *GBCpu) { panic("Opcode 46 unimplemented") },          // 46
	func(cpu *GBCpu) { panic("Opcode 47 unimplemented") },          // 47
	func(cpu *GBCpu) { panic("Opcode 48 unimplemented") },          // 48
	func(cpu *GBCpu) { panic("Opcode 49 unimplemented") },          // 49
	func(cpu *GBCpu) { panic("Opcode 4A unimplemented") },          // 4A
	func(cpu *GBCpu) { panic("Opcode 4B unimplemented") },          // 4B
	func(cpu *GBCpu) { panic("Opcode 4C unimplemented") },          // 4C
	func(cpu *GBCpu) { panic("Opcode 4D unimplemented") },          // 4D
	func(cpu *GBCpu) { panic("Opcode 4E unimplemented") },          // 4E
	func(cpu *GBCpu) { panic("Opcode 4F unimplemented") },          // 4F
	func(cpu *GBCpu) { panic("Opcode 50 unimplemented") },          // 50
	func(cpu *GBCpu) { panic("Opcode 51 unimplemented") },          // 51
	func(cpu *GBCpu) { panic("Opcode 52 unimplemented") },          // 52
	func(cpu *GBCpu) { panic("Opcode 53 unimplemented") },          // 53
	func(cpu *GBCpu) { panic("Opcode 54 unimplemented") },          // 54
	func(cpu *GBCpu) { panic("Opcode 55 unimplemented") },          // 55
	func(cpu *GBCpu) { panic("Opcode 56 unimplemented") },          // 56
	func(cpu *GBCpu) { panic("Opcode 57 unimplemented") },          // 57
	func(cpu *GBCpu) { panic("Opcode 58 unimplemented") },          // 58
	func(cpu *GBCpu) { panic("Opcode 59 unimplemented") },          // 59
	func(cpu *GBCpu) { panic("Opcode 5A unimplemented") },          // 5A
	func(cpu *GBCpu) { panic("Opcode 5B unimplemented") },          // 5B
	func(cpu *GBCpu) { panic("Opcode 5C unimplemented") },          // 5C
	func(cpu *GBCpu) { panic("Opcode 5D unimplemented") },          // 5D
	func(cpu *GBCpu) { panic("Opcode 5E unimplemented") },          // 5E
	func(cpu *GBCpu) { panic("Opcode 5F unimplemented") },          // 5F
	func(cpu *GBCpu) { panic("Opcode 60 unimplemented") },          // 60
	func(cpu *GBCpu) { panic("Opcode 61 unimplemented") },          // 61
	func(cpu *GBCpu) { panic("Opcode 62 unimplemented") },          // 62
	func(cpu *GBCpu) { panic("Opcode 63 unimplemented") },          // 63
	func(cpu *GBCpu) { panic("Opcode 64 unimplemented") },          // 64
	func(cpu *GBCpu) { panic("Opcode 65 unimplemented") },          // 65
	func(cpu *GBCpu) { panic("Opcode 66 unimplemented") },          // 66
	func(cpu *GBCpu) { panic("Opcode 67 unimplemented") },          // 67
	func(cpu *GBCpu) { panic("Opcode 68 unimplemented") },          // 68
	func(cpu *GBCpu) { panic("Opcode 69 unimplemented") },          // 69
	func(cpu *GBCpu) { panic("Opcode 6A unimplemented") },          // 6A
	func(cpu *GBCpu) { panic("Opcode 6B unimplemented") },          // 6B
	func(cpu *GBCpu) { panic("Opcode 6C unimplemented") },          // 6C
	func(cpu *GBCpu) { panic("Opcode 6D unimplemented") },          // 6D
	func(cpu *GBCpu) { panic("Opcode 6E unimplemented") },          // 6E
	func(cpu *GBCpu) { panic("Opcode 6F unimplemented") },          // 6F
	func(cpu *GBCpu) { panic("Opcode 70 unimplemented") },          // 70
	func(cpu *GBCpu) { panic("Opcode 71 unimplemented") },          // 71
	func(cpu *GBCpu) { panic("Opcode 72 unimplemented") },          // 72
	func(cpu *GBCpu) { panic("Opcode 73 unimplemented") },          // 73
	func(cpu *GBCpu) { panic("Opcode 74 unimplemented") },          // 74
	func(cpu *GBCpu) { panic("Opcode 75 unimplemented") },          // 75
	func(cpu *GBCpu) { panic("Opcode 76 unimplemented") },          // 76
	func(cpu *GBCpu) { panic("Opcode 77 unimplemented") },          // 77
	func(cpu *GBCpu) { panic("Opcode 78 unimplemented") },          // 78
	func(cpu *GBCpu) { panic("Opcode 79 unimplemented") },          // 79
	func(cpu *GBCpu) { panic("Opcode 7A unimplemented") },          // 7A
	func(cpu *GBCpu) { panic("Opcode 7B unimplemented") },          // 7B
	func(cpu *GBCpu) { panic("Opcode 7C unimplemented") },          // 7C
	func(cpu *GBCpu) { panic("Opcode 7D unimplemented") },          // 7D
	func(cpu *GBCpu) { panic("Opcode 7E unimplemented") },          // 7E
	func(cpu *GBCpu) { panic("Opcode 7F unimplemented") },          // 7F
	func(cpu *GBCpu) { handler_add_a(cpu, cpu.b) },                 // 80
	func(cpu *GBCpu) { handler_add_a(cpu, cpu.c) },                 // 81
	func(cpu *GBCpu) { handler_add_a(cpu, cpu.d) },                 // 82
	func(cpu *GBCpu) { handler_add_a(cpu, cpu.e) },                 // 83
	func(cpu *GBCpu) { handler_add_a(cpu, cpu.h) },                 // 84
	func(cpu *GBCpu) { handler_add_a(cpu, cpu.l) },                 // 85
	func(cpu *GBCpu) { panic("Opcode 86 unimplemented") },          // 86
	func(cpu *GBCpu) { panic("Opcode 87 unimplemented") },          // 87
	func(cpu *GBCpu) { panic("Opcode 88 unimplemented") },          // 88
	func(cpu *GBCpu) { panic("Opcode 89 unimplemented") },          // 89
	func(cpu *GBCpu) { panic("Opcode 8A unimplemented") },          // 8A
	func(cpu *GBCpu) { panic("Opcode 8B unimplemented") },          // 8B
	func(cpu *GBCpu) { panic("Opcode 8C unimplemented") },          // 8C
	func(cpu *GBCpu) { panic("Opcode 8D unimplemented") },          // 8D
	func(cpu *GBCpu) { panic("Opcode 8E unimplemented") },          // 8E
	func(cpu *GBCpu) { panic("Opcode 8F unimplemented") },          // 8F
	func(cpu *GBCpu) { panic("Opcode 90 unimplemented") },          // 90
	func(cpu *GBCpu) { panic("Opcode 91 unimplemented") },          // 91
	func(cpu *GBCpu) { panic("Opcode 92 unimplemented") },          // 92
	func(cpu *GBCpu) { panic("Opcode 93 unimplemented") },          // 93
	func(cpu *GBCpu) { panic("Opcode 94 unimplemented") },          // 94
	func(cpu *GBCpu) { panic("Opcode 95 unimplemented") },          // 95
	func(cpu *GBCpu) { panic("Opcode 96 unimplemented") },          // 96
	func(cpu *GBCpu) { panic("Opcode 97 unimplemented") },          // 97
	func(cpu *GBCpu) { panic("Opcode 98 unimplemented") },          // 98
	func(cpu *GBCpu) { panic("Opcode 99 unimplemented") },          // 99
	func(cpu *GBCpu) { panic("Opcode 9A unimplemented") },          // 9A
	func(cpu *GBCpu) { panic("Opcode 9B unimplemented") },          // 9B
	func(cpu *GBCpu) { panic("Opcode 9C unimplemented") },          // 9C
	func(cpu *GBCpu) { panic("Opcode 9D unimplemented") },          // 9D
	func(cpu *GBCpu) { panic("Opcode 9E unimplemented") },          // 9E
	func(cpu *GBCpu) { panic("Opcode 9F unimplemented") },          // 9F
	func(cpu *GBCpu) { panic("Opcode A0 unimplemented") },          // A0
	func(cpu *GBCpu) { panic("Opcode A1 unimplemented") },          // A1
	func(cpu *GBCpu) { panic("Opcode A2 unimplemented") },          // A2
	func(cpu *GBCpu) { panic("Opcode A3 unimplemented") },          // A3
	func(cpu *GBCpu) { panic("Opcode A4 unimplemented") },          // A4
	func(cpu *GBCpu) { panic("Opcode A5 unimplemented") },          // A5
	func(cpu *GBCpu) { panic("Opcode A6 unimplemented") },          // A6
	func(cpu *GBCpu) { panic("Opcode A7 unimplemented") },          // A7
	func(cpu *GBCpu) { panic("Opcode A8 unimplemented") },          // A8
	func(cpu *GBCpu) { panic("Opcode A9 unimplemented") },          // A9
	func(cpu *GBCpu) { panic("Opcode AA unimplemented") },          // AA
	func(cpu *GBCpu) { panic("Opcode AB unimplemented") },          // AB
	func(cpu *GBCpu) { panic("Opcode AC unimplemented") },          // AC
	func(cpu *GBCpu) { panic("Opcode AD unimplemented") },          // AD
	func(cpu *GBCpu) { panic("Opcode AE unimplemented") },          // AE
	func(cpu *GBCpu) { panic("Opcode AF unimplemented") },          // AF
	func(cpu *GBCpu) { panic("Opcode B0 unimplemented") },          // B0
	func(cpu *GBCpu) { panic("Opcode B1 unimplemented") },          // B1
	func(cpu *GBCpu) { panic("Opcode B2 unimplemented") },          // B2
	func(cpu *GBCpu) { panic("Opcode B3 unimplemented") },          // B3
	func(cpu *GBCpu) { panic("Opcode B4 unimplemented") },          // B4
	func(cpu *GBCpu) { panic("Opcode B5 unimplemented") },          // B5
	func(cpu *GBCpu) { panic("Opcode B6 unimplemented") },          // B6
	func(cpu *GBCpu) { panic("Opcode B7 unimplemented") },          // B7
	func(cpu *GBCpu) { panic("Opcode B8 unimplemented") },          // B8
	func(cpu *GBCpu) { panic("Opcode B9 unimplemented") },          // B9
	func(cpu *GBCpu) { panic("Opcode BA unimplemented") },          // BA
	func(cpu *GBCpu) { panic("Opcode BB unimplemented") },          // BB
	func(cpu *GBCpu) { panic("Opcode BC unimplemented") },          // BC
	func(cpu *GBCpu) { panic("Opcode BD unimplemented") },          // BD
	func(cpu *GBCpu) { panic("Opcode BE unimplemented") },          // BE
	func(cpu *GBCpu) { panic("Opcode BF unimplemented") },          // BF
	func(cpu *GBCpu) { panic("Opcode C0 unimplemented") },          // C0
	func(cpu *GBCpu) { panic("Opcode C1 unimplemented") },          // C1
	func(cpu *GBCpu) { panic("Opcode C2 unimplemented") },          // C2
	func(cpu *GBCpu) { panic("Opcode C3 unimplemented") },          // C3
	func(cpu *GBCpu) { panic("Opcode C4 unimplemented") },          // C4
	func(cpu *GBCpu) { panic("Opcode C5 unimplemented") },          // C5
	func(cpu *GBCpu) { handler_add_a(cpu, cpu.getPC8()) },          // C6
	func(cpu *GBCpu) { panic("Opcode C7 unimplemented") },          // C7
	func(cpu *GBCpu) { panic("Opcode C8 unimplemented") },          // C8
	func(cpu *GBCpu) { panic("Opcode C9 unimplemented") },          // C9
	func(cpu *GBCpu) { panic("Opcode CA unimplemented") },          // CA
	func(cpu *GBCpu) { panic("Opcode CB unimplemented") },          // CB
	func(cpu *GBCpu) { panic("Opcode CC unimplemented") },          // CC
	func(cpu *GBCpu) { panic("Opcode CD unimplemented") },          // CD
	func(cpu *GBCpu) { panic("Opcode CE unimplemented") },          // CE
	func(cpu *GBCpu) { panic("Opcode CF unimplemented") },          // CF
	func(cpu *GBCpu) { panic("Opcode D0 unimplemented") },          // D0
	func(cpu *GBCpu) { panic("Opcode D1 unimplemented") },          // D1
	func(cpu *GBCpu) { panic("Opcode D2 unimplemented") },          // D2
	func(cpu *GBCpu) { panic("Opcode D3 unimplemented") },          // D3
	func(cpu *GBCpu) { panic("Opcode D4 unimplemented") },          // D4
	func(cpu *GBCpu) { panic("Opcode D5 unimplemented") },          // D5
	func(cpu *GBCpu) { panic("Opcode D6 unimplemented") },          // D6
	func(cpu *GBCpu) { panic("Opcode D7 unimplemented") },          // D7
	func(cpu *GBCpu) { panic("Opcode D8 unimplemented") },          // D8
	func(cpu *GBCpu) { panic("Opcode D9 unimplemented") },          // D9
	func(cpu *GBCpu) { panic("Opcode DA unimplemented") },          // DA
	func(cpu *GBCpu) { panic("Opcode DB unimplemented") },          // DB
	func(cpu *GBCpu) { panic("Opcode DC unimplemented") },          // DC
	func(cpu *GBCpu) { panic("Opcode DD unimplemented") },          // DD
	func(cpu *GBCpu) { panic("Opcode DE unimplemented") },          // DE
	func(cpu *GBCpu) { panic("Opcode DF unimplemented") },          // DF
	func(cpu *GBCpu) { panic("Opcode E0 unimplemented") },          // E0
	func(cpu *GBCpu) { panic("Opcode E1 unimplemented") },          // E1
	func(cpu *GBCpu) { panic("Opcode E2 unimplemented") },          // E2
	func(cpu *GBCpu) { panic("Opcode E3 unimplemented") },          // E3
	func(cpu *GBCpu) { panic("Opcode E4 unimplemented") },          // E4
	func(cpu *GBCpu) { panic("Opcode E5 unimplemented") },          // E5
	func(cpu *GBCpu) { panic("Opcode E6 unimplemented") },          // E6
	func(cpu *GBCpu) { panic("Opcode E7 unimplemented") },          // E7
	func(cpu *GBCpu) { panic("Opcode E8 unimplemented") },          // E8
	func(cpu *GBCpu) { panic("Opcode E9 unimplemented") },          // E9
	func(cpu *GBCpu) { panic("Opcode EA unimplemented") },          // EA
	func(cpu *GBCpu) { panic("Opcode EB unimplemented") },          // EB
	func(cpu *GBCpu) { panic("Opcode EC unimplemented") },          // EC
	func(cpu *GBCpu) { panic("Opcode ED unimplemented") },          // ED
	func(cpu *GBCpu) { panic("Opcode EE unimplemented") },          // EE
	func(cpu *GBCpu) { panic("Opcode EF unimplemented") },          // EF
	func(cpu *GBCpu) { panic("Opcode F0 unimplemented") },          // F0
	func(cpu *GBCpu) { panic("Opcode F1 unimplemented") },          // F1
	func(cpu *GBCpu) { panic("Opcode F2 unimplemented") },          // F2
	func(cpu *GBCpu) { panic("Opcode F3 unimplemented") },          // F3
	func(cpu *GBCpu) { panic("Opcode F4 unimplemented") },          // F4
	func(cpu *GBCpu) { panic("Opcode F5 unimplemented") },          // F5
	func(cpu *GBCpu) { panic("Opcode F6 unimplemented") },          // F6
	func(cpu *GBCpu) { panic("Opcode F7 unimplemented") },          // F7
	func(cpu *GBCpu) { panic("Opcode F8 unimplemented") },          // F8
	func(cpu *GBCpu) { panic("Opcode F9 unimplemented") },          // F9
	func(cpu *GBCpu) { panic("Opcode FA unimplemented") },          // FA
	func(cpu *GBCpu) { panic("Opcode FB unimplemented") },          // FB
	func(cpu *GBCpu) { panic("Opcode FC unimplemented") },          // FC
	func(cpu *GBCpu) { panic("Opcode FD unimplemented") },          // FD
	func(cpu *GBCpu) { panic("Opcode FE unimplemented") },          // FE
	func(cpu *GBCpu) { panic("Opcode FF unimplemented") },          // FF
}
