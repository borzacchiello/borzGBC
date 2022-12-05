package z80cpu

type Memory interface {
	Read(uint16) uint8
	Write(uint16, uint8)
}

type Z80Cpu struct {
	Mem                 Memory
	a, b, c, d, e, h, l uint8
	sp, pc              uint16

	flagWasZero, flagWasSub, flagHalfCarry, flagCarry bool

	branchWasTaken bool
}

type Condition uint8

const (
	Condition_C  Condition = 1
	Condition_NC           = 2
	Condition_Z            = 3
	Condition_NZ           = 4
)

func MakeZ80Cpu(mem Memory) *Z80Cpu {
	return &Z80Cpu{
		Mem: mem,
	}
}

func (cpu *Z80Cpu) fetchOpcode() uint8 {
	opcode := cpu.Mem.Read(cpu.pc)
	cpu.pc += 1
	return opcode
}

func (cpu *Z80Cpu) evalCondition(cond Condition) bool {
	var res bool

	switch cond {
	case Condition_C:
		res = cpu.flagCarry
	case Condition_NC:
		res = !cpu.flagCarry
	case Condition_Z:
		res = cpu.flagWasZero
	case Condition_NZ:
		res = !cpu.flagWasZero
	default:
		panic("evalCondition: invalid condition")
	}

	cpu.branchWasTaken = res
	return res
}

func (cpu *Z80Cpu) ExecOne() {
	opcode := cpu.fetchOpcode()
	handler := handlers[opcode]

	handler(cpu)
}

func (cpu *Z80Cpu) PackFlags() uint8 {
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

func (cpu *Z80Cpu) UnpackFlags(f uint8) {
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

func (cpu *Z80Cpu) getPC8() uint8 {
	v := cpu.Mem.Read(cpu.pc)

	cpu.pc += 1
	return v
}

func (cpu *Z80Cpu) getPC16() uint16 {
	l := cpu.Mem.Read(cpu.pc)
	h := cpu.Mem.Read(cpu.pc + 1)

	cpu.pc += 2
	return (uint16(h) << 8) | uint16(l)
}

func pack_regcouple(b, a uint8) uint16 {
	return (uint16(b) << 8) | uint16(a)
}

func unpack_regcouple(a uint16) (uint8, uint8) {
	return uint8(a >> 8 & 0xff), uint8(a & 0xff)
}

func handler_nop() {}

func handler_stop() {}

func handler_ld_R_PC_16(cpu *Z80Cpu, dst1, dst2 *uint8) {
	*dst1, *dst2 = unpack_regcouple(cpu.getPC16())
}

func handler_ld_R_PC_8(cpu *Z80Cpu, dst *uint8) {
	*dst = cpu.getPC8()
}

func handler_ld_MEM_8(cpu *Z80Cpu, dst_addr uint16, src uint8) {
	cpu.Mem.Write(dst_addr, src)
}

func handler_ld_MEM_16(cpu *Z80Cpu, dst_addr uint16, src uint16) {
	cpu.Mem.Write(dst_addr, uint8(src&0xff))
	cpu.Mem.Write(dst_addr+1, uint8((src>>8)&0xff))
}

func handler_ld_R_MEM_8(cpu *Z80Cpu, dst *uint8, addr uint16) {
	*dst = cpu.Mem.Read(addr)
}

func handler_inc_R_8(cpu *Z80Cpu, dst *uint8) {
	*dst = *dst + 1

	cpu.flagWasZero = *dst == 0
	cpu.flagWasSub = false
	cpu.flagHalfCarry = *dst&0xf == 0
}

func handler_inc_R_16(cpu *Z80Cpu, dst1, dst2 *uint8) {
	v := pack_regcouple(*dst1, *dst2)
	v += 1
	*dst1, *dst2 = unpack_regcouple(v)
}

func handler_dec_R_8(cpu *Z80Cpu, dst *uint8) {
	*dst = *dst - 1

	cpu.flagWasZero = *dst == 0
	cpu.flagWasSub = true
	cpu.flagHalfCarry = *dst&0xf == 0xf
}

func handler_dec_R_16(cpu *Z80Cpu, dst1, dst2 *uint8) {
	dst := pack_regcouple(*dst1, *dst2)
	dst -= 1
	*dst1, *dst2 = unpack_regcouple(dst)
}

func handler_add_R_R_8(cpu *Z80Cpu, dst *uint8, src uint8) {
	old_dst := *dst
	res := uint16(old_dst) + uint16(src)
	*dst = uint8(res & 0xff)

	cpu.flagWasZero = *dst == 0
	cpu.flagWasSub = false
	cpu.flagHalfCarry = (old_dst&0xf)+(src&0xf) > 0xf
	cpu.flagCarry = res&0x100 != 0
}

func handler_add_R_R_16(cpu *Z80Cpu, dst1, dst2 *uint8, src uint16) {
	dst := pack_regcouple(*dst1, *dst2)
	res := dst + src
	*dst1, *dst2 = unpack_regcouple(res)

	cpu.flagWasSub = false
	cpu.flagHalfCarry = ((dst&0xfff)+(src&0xfff) > 0xfff)
	cpu.flagCarry = int(dst)+int(src) > 0xffff
}

func handler_add_R_PC(cpu *Z80Cpu, dst *uint8) {
	src := cpu.getPC8()
	handler_add_R_R_8(cpu, dst, src)
}

func handler_rlc_R(cpu *Z80Cpu, dst *uint8) {
	carry := *dst >> 7
	res := *dst<<1 | carry

	cpu.flagWasZero = res == 0
	cpu.flagWasSub = false
	cpu.flagHalfCarry = false
	cpu.flagCarry = carry != 0
}

func handler_rrc_R(cpu *Z80Cpu, dst *uint8) {
	carry := *dst & 1
	res := carry | *dst>>1

	cpu.flagWasZero = res == 0
	cpu.flagWasSub = false
	cpu.flagHalfCarry = false
	cpu.flagCarry = carry != 0
}

func handler_rl_R(cpu *Z80Cpu, dst *uint8) {
	carry := *dst >> 7

	*dst = *dst << 1
	if cpu.flagCarry {
		*dst |= 1
	}

	cpu.flagWasZero = *dst == 0
	cpu.flagWasSub = false
	cpu.flagHalfCarry = false
	cpu.flagCarry = carry != 0
}

func handler_rr_R(cpu *Z80Cpu, dst *uint8) {
	carry := *dst & 1

	*dst = *dst >> 1
	if cpu.flagCarry {
		*dst |= 1 << 7
	}

	cpu.flagWasZero = *dst == 0
	cpu.flagWasSub = false
	cpu.flagHalfCarry = false
	cpu.flagCarry = carry != 0
}

func handler_jr(cpu *Z80Cpu, off int8) {
	cpu.pc = uint16(int(cpu.pc) + int(off))
}

func handler_jr_IF(cpu *Z80Cpu, off int8, cond bool) {

}

var handlers = [256]func(*Z80Cpu){
	func(cpu *Z80Cpu) { handler_nop() },                                                         // 00
	func(cpu *Z80Cpu) { handler_ld_R_PC_16(cpu, &cpu.b, &cpu.c) },                               // 01
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.b, cpu.c), cpu.a) },            // 02
	func(cpu *Z80Cpu) { handler_inc_R_16(cpu, &cpu.b, &cpu.c) },                                 // 03
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.b) },                                          // 04
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.b) },                                          // 05
	func(cpu *Z80Cpu) { handler_ld_R_PC_8(cpu, &cpu.b) },                                        // 06
	func(cpu *Z80Cpu) { handler_rlc_R(cpu, &cpu.a); cpu.flagWasZero = false /* rlca */ },        // 07
	func(cpu *Z80Cpu) { handler_ld_MEM_16(cpu, cpu.getPC16(), cpu.sp) },                         // 08
	func(cpu *Z80Cpu) { handler_add_R_R_16(cpu, &cpu.h, &cpu.l, pack_regcouple(cpu.b, cpu.c)) }, // 09
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.a, pack_regcouple(cpu.b, cpu.c)) },         // 0A
	func(cpu *Z80Cpu) { handler_dec_R_16(cpu, &cpu.b, &cpu.c) },                                 // 0B
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.c) },                                          // 0C
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.c) },                                          // 0D
	func(cpu *Z80Cpu) { handler_ld_R_PC_8(cpu, &cpu.c) },                                        // 0E
	func(cpu *Z80Cpu) { handler_rrc_R(cpu, &cpu.a); cpu.flagWasZero = false /* rrca */ },        // 0F
	func(cpu *Z80Cpu) { handler_stop() },                                                        // 10
	func(cpu *Z80Cpu) { handler_ld_R_PC_16(cpu, &cpu.d, &cpu.e) },                               // 11
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.d, cpu.e), cpu.a) },            // 12
	func(cpu *Z80Cpu) { handler_inc_R_16(cpu, &cpu.d, &cpu.e) },                                 // 13
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.d) },                                          // 14
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.d) },                                          // 15
	func(cpu *Z80Cpu) { handler_ld_R_PC_8(cpu, &cpu.d) },                                        // 16
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.a); cpu.flagWasZero = false /* rla */ },          // 17
	func(cpu *Z80Cpu) { handler_jr(cpu, int8(cpu.Mem.Read(cpu.pc))) },                           // 18
	func(cpu *Z80Cpu) { handler_add_R_R_16(cpu, &cpu.h, &cpu.l, pack_regcouple(cpu.d, cpu.e)) }, // 19
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.a, pack_regcouple(cpu.d, cpu.e)) },         // 1A
	func(cpu *Z80Cpu) { handler_dec_R_16(cpu, &cpu.d, &cpu.e) },                                 // 1B
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.e) },                                          // 1C
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.e) },                                          // 1D
	func(cpu *Z80Cpu) { handler_ld_R_PC_8(cpu, &cpu.e) },                                        // 1E
	func(cpu *Z80Cpu) { handler_rr_R(cpu, &cpu.a); cpu.flagWasZero = false /* rra */ },          // 1F
	func(cpu *Z80Cpu) { panic("Opcode 20 unimplemented") },                                      // 20
	func(cpu *Z80Cpu) { panic("Opcode 21 unimplemented") },                                      // 21
	func(cpu *Z80Cpu) { panic("Opcode 22 unimplemented") },                                      // 22
	func(cpu *Z80Cpu) { panic("Opcode 23 unimplemented") },                                      // 23
	func(cpu *Z80Cpu) { panic("Opcode 24 unimplemented") },                                      // 24
	func(cpu *Z80Cpu) { panic("Opcode 25 unimplemented") },                                      // 25
	func(cpu *Z80Cpu) { panic("Opcode 26 unimplemented") },                                      // 26
	func(cpu *Z80Cpu) { panic("Opcode 27 unimplemented") },                                      // 27
	func(cpu *Z80Cpu) { panic("Opcode 28 unimplemented") },                                      // 28
	func(cpu *Z80Cpu) { panic("Opcode 29 unimplemented") },                                      // 29
	func(cpu *Z80Cpu) { panic("Opcode 2A unimplemented") },                                      // 2A
	func(cpu *Z80Cpu) { panic("Opcode 2B unimplemented") },                                      // 2B
	func(cpu *Z80Cpu) { panic("Opcode 2C unimplemented") },                                      // 2C
	func(cpu *Z80Cpu) { panic("Opcode 2D unimplemented") },                                      // 2D
	func(cpu *Z80Cpu) { panic("Opcode 2E unimplemented") },                                      // 2E
	func(cpu *Z80Cpu) { panic("Opcode 2F unimplemented") },                                      // 2F
	func(cpu *Z80Cpu) { panic("Opcode 30 unimplemented") },                                      // 30
	func(cpu *Z80Cpu) { panic("Opcode 31 unimplemented") },                                      // 31
	func(cpu *Z80Cpu) { panic("Opcode 32 unimplemented") },                                      // 32
	func(cpu *Z80Cpu) { panic("Opcode 33 unimplemented") },                                      // 33
	func(cpu *Z80Cpu) { panic("Opcode 34 unimplemented") },                                      // 34
	func(cpu *Z80Cpu) { panic("Opcode 35 unimplemented") },                                      // 35
	func(cpu *Z80Cpu) { panic("Opcode 36 unimplemented") },                                      // 36
	func(cpu *Z80Cpu) { panic("Opcode 37 unimplemented") },                                      // 37
	func(cpu *Z80Cpu) { panic("Opcode 38 unimplemented") },                                      // 38
	func(cpu *Z80Cpu) { panic("Opcode 39 unimplemented") },                                      // 39
	func(cpu *Z80Cpu) { panic("Opcode 3A unimplemented") },                                      // 3A
	func(cpu *Z80Cpu) { panic("Opcode 3B unimplemented") },                                      // 3B
	func(cpu *Z80Cpu) { panic("Opcode 3C unimplemented") },                                      // 3C
	func(cpu *Z80Cpu) { panic("Opcode 3D unimplemented") },                                      // 3D
	func(cpu *Z80Cpu) { handler_ld_R_PC_8(cpu, &cpu.a) },                                        // 3E
	func(cpu *Z80Cpu) { panic("Opcode 3F unimplemented") },                                      // 3F
	func(cpu *Z80Cpu) { panic("Opcode 40 unimplemented") },                                      // 40
	func(cpu *Z80Cpu) { panic("Opcode 41 unimplemented") },                                      // 41
	func(cpu *Z80Cpu) { panic("Opcode 42 unimplemented") },                                      // 42
	func(cpu *Z80Cpu) { panic("Opcode 43 unimplemented") },                                      // 43
	func(cpu *Z80Cpu) { panic("Opcode 44 unimplemented") },                                      // 44
	func(cpu *Z80Cpu) { panic("Opcode 45 unimplemented") },                                      // 45
	func(cpu *Z80Cpu) { panic("Opcode 46 unimplemented") },                                      // 46
	func(cpu *Z80Cpu) { panic("Opcode 47 unimplemented") },                                      // 47
	func(cpu *Z80Cpu) { panic("Opcode 48 unimplemented") },                                      // 48
	func(cpu *Z80Cpu) { panic("Opcode 49 unimplemented") },                                      // 49
	func(cpu *Z80Cpu) { panic("Opcode 4A unimplemented") },                                      // 4A
	func(cpu *Z80Cpu) { panic("Opcode 4B unimplemented") },                                      // 4B
	func(cpu *Z80Cpu) { panic("Opcode 4C unimplemented") },                                      // 4C
	func(cpu *Z80Cpu) { panic("Opcode 4D unimplemented") },                                      // 4D
	func(cpu *Z80Cpu) { panic("Opcode 4E unimplemented") },                                      // 4E
	func(cpu *Z80Cpu) { panic("Opcode 4F unimplemented") },                                      // 4F
	func(cpu *Z80Cpu) { panic("Opcode 50 unimplemented") },                                      // 50
	func(cpu *Z80Cpu) { panic("Opcode 51 unimplemented") },                                      // 51
	func(cpu *Z80Cpu) { panic("Opcode 52 unimplemented") },                                      // 52
	func(cpu *Z80Cpu) { panic("Opcode 53 unimplemented") },                                      // 53
	func(cpu *Z80Cpu) { panic("Opcode 54 unimplemented") },                                      // 54
	func(cpu *Z80Cpu) { panic("Opcode 55 unimplemented") },                                      // 55
	func(cpu *Z80Cpu) { panic("Opcode 56 unimplemented") },                                      // 56
	func(cpu *Z80Cpu) { panic("Opcode 57 unimplemented") },                                      // 57
	func(cpu *Z80Cpu) { panic("Opcode 58 unimplemented") },                                      // 58
	func(cpu *Z80Cpu) { panic("Opcode 59 unimplemented") },                                      // 59
	func(cpu *Z80Cpu) { panic("Opcode 5A unimplemented") },                                      // 5A
	func(cpu *Z80Cpu) { panic("Opcode 5B unimplemented") },                                      // 5B
	func(cpu *Z80Cpu) { panic("Opcode 5C unimplemented") },                                      // 5C
	func(cpu *Z80Cpu) { panic("Opcode 5D unimplemented") },                                      // 5D
	func(cpu *Z80Cpu) { panic("Opcode 5E unimplemented") },                                      // 5E
	func(cpu *Z80Cpu) { panic("Opcode 5F unimplemented") },                                      // 5F
	func(cpu *Z80Cpu) { panic("Opcode 60 unimplemented") },                                      // 60
	func(cpu *Z80Cpu) { panic("Opcode 61 unimplemented") },                                      // 61
	func(cpu *Z80Cpu) { panic("Opcode 62 unimplemented") },                                      // 62
	func(cpu *Z80Cpu) { panic("Opcode 63 unimplemented") },                                      // 63
	func(cpu *Z80Cpu) { panic("Opcode 64 unimplemented") },                                      // 64
	func(cpu *Z80Cpu) { panic("Opcode 65 unimplemented") },                                      // 65
	func(cpu *Z80Cpu) { panic("Opcode 66 unimplemented") },                                      // 66
	func(cpu *Z80Cpu) { panic("Opcode 67 unimplemented") },                                      // 67
	func(cpu *Z80Cpu) { panic("Opcode 68 unimplemented") },                                      // 68
	func(cpu *Z80Cpu) { panic("Opcode 69 unimplemented") },                                      // 69
	func(cpu *Z80Cpu) { panic("Opcode 6A unimplemented") },                                      // 6A
	func(cpu *Z80Cpu) { panic("Opcode 6B unimplemented") },                                      // 6B
	func(cpu *Z80Cpu) { panic("Opcode 6C unimplemented") },                                      // 6C
	func(cpu *Z80Cpu) { panic("Opcode 6D unimplemented") },                                      // 6D
	func(cpu *Z80Cpu) { panic("Opcode 6E unimplemented") },                                      // 6E
	func(cpu *Z80Cpu) { panic("Opcode 6F unimplemented") },                                      // 6F
	func(cpu *Z80Cpu) { panic("Opcode 70 unimplemented") },                                      // 70
	func(cpu *Z80Cpu) { panic("Opcode 71 unimplemented") },                                      // 71
	func(cpu *Z80Cpu) { panic("Opcode 72 unimplemented") },                                      // 72
	func(cpu *Z80Cpu) { panic("Opcode 73 unimplemented") },                                      // 73
	func(cpu *Z80Cpu) { panic("Opcode 74 unimplemented") },                                      // 74
	func(cpu *Z80Cpu) { panic("Opcode 75 unimplemented") },                                      // 75
	func(cpu *Z80Cpu) { panic("Opcode 76 unimplemented") },                                      // 76
	func(cpu *Z80Cpu) { panic("Opcode 77 unimplemented") },                                      // 77
	func(cpu *Z80Cpu) { panic("Opcode 78 unimplemented") },                                      // 78
	func(cpu *Z80Cpu) { panic("Opcode 79 unimplemented") },                                      // 79
	func(cpu *Z80Cpu) { panic("Opcode 7A unimplemented") },                                      // 7A
	func(cpu *Z80Cpu) { panic("Opcode 7B unimplemented") },                                      // 7B
	func(cpu *Z80Cpu) { panic("Opcode 7C unimplemented") },                                      // 7C
	func(cpu *Z80Cpu) { panic("Opcode 7D unimplemented") },                                      // 7D
	func(cpu *Z80Cpu) { panic("Opcode 7E unimplemented") },                                      // 7E
	func(cpu *Z80Cpu) { panic("Opcode 7F unimplemented") },                                      // 7F
	func(cpu *Z80Cpu) { handler_add_R_R_8(cpu, &cpu.a, cpu.b) },                                 // 80
	func(cpu *Z80Cpu) { handler_add_R_R_8(cpu, &cpu.a, cpu.c) },                                 // 81
	func(cpu *Z80Cpu) { handler_add_R_R_8(cpu, &cpu.a, cpu.d) },                                 // 82
	func(cpu *Z80Cpu) { handler_add_R_R_8(cpu, &cpu.a, cpu.e) },                                 // 83
	func(cpu *Z80Cpu) { handler_add_R_R_8(cpu, &cpu.a, cpu.h) },                                 // 84
	func(cpu *Z80Cpu) { handler_add_R_R_8(cpu, &cpu.a, cpu.l) },                                 // 85
	func(cpu *Z80Cpu) { panic("Opcode 86 unimplemented") },                                      // 86
	func(cpu *Z80Cpu) { panic("Opcode 87 unimplemented") },                                      // 87
	func(cpu *Z80Cpu) { panic("Opcode 88 unimplemented") },                                      // 88
	func(cpu *Z80Cpu) { panic("Opcode 89 unimplemented") },                                      // 89
	func(cpu *Z80Cpu) { panic("Opcode 8A unimplemented") },                                      // 8A
	func(cpu *Z80Cpu) { panic("Opcode 8B unimplemented") },                                      // 8B
	func(cpu *Z80Cpu) { panic("Opcode 8C unimplemented") },                                      // 8C
	func(cpu *Z80Cpu) { panic("Opcode 8D unimplemented") },                                      // 8D
	func(cpu *Z80Cpu) { panic("Opcode 8E unimplemented") },                                      // 8E
	func(cpu *Z80Cpu) { panic("Opcode 8F unimplemented") },                                      // 8F
	func(cpu *Z80Cpu) { panic("Opcode 90 unimplemented") },                                      // 90
	func(cpu *Z80Cpu) { panic("Opcode 91 unimplemented") },                                      // 91
	func(cpu *Z80Cpu) { panic("Opcode 92 unimplemented") },                                      // 92
	func(cpu *Z80Cpu) { panic("Opcode 93 unimplemented") },                                      // 93
	func(cpu *Z80Cpu) { panic("Opcode 94 unimplemented") },                                      // 94
	func(cpu *Z80Cpu) { panic("Opcode 95 unimplemented") },                                      // 95
	func(cpu *Z80Cpu) { panic("Opcode 96 unimplemented") },                                      // 96
	func(cpu *Z80Cpu) { panic("Opcode 97 unimplemented") },                                      // 97
	func(cpu *Z80Cpu) { panic("Opcode 98 unimplemented") },                                      // 98
	func(cpu *Z80Cpu) { panic("Opcode 99 unimplemented") },                                      // 99
	func(cpu *Z80Cpu) { panic("Opcode 9A unimplemented") },                                      // 9A
	func(cpu *Z80Cpu) { panic("Opcode 9B unimplemented") },                                      // 9B
	func(cpu *Z80Cpu) { panic("Opcode 9C unimplemented") },                                      // 9C
	func(cpu *Z80Cpu) { panic("Opcode 9D unimplemented") },                                      // 9D
	func(cpu *Z80Cpu) { panic("Opcode 9E unimplemented") },                                      // 9E
	func(cpu *Z80Cpu) { panic("Opcode 9F unimplemented") },                                      // 9F
	func(cpu *Z80Cpu) { panic("Opcode A0 unimplemented") },                                      // A0
	func(cpu *Z80Cpu) { panic("Opcode A1 unimplemented") },                                      // A1
	func(cpu *Z80Cpu) { panic("Opcode A2 unimplemented") },                                      // A2
	func(cpu *Z80Cpu) { panic("Opcode A3 unimplemented") },                                      // A3
	func(cpu *Z80Cpu) { panic("Opcode A4 unimplemented") },                                      // A4
	func(cpu *Z80Cpu) { panic("Opcode A5 unimplemented") },                                      // A5
	func(cpu *Z80Cpu) { panic("Opcode A6 unimplemented") },                                      // A6
	func(cpu *Z80Cpu) { panic("Opcode A7 unimplemented") },                                      // A7
	func(cpu *Z80Cpu) { panic("Opcode A8 unimplemented") },                                      // A8
	func(cpu *Z80Cpu) { panic("Opcode A9 unimplemented") },                                      // A9
	func(cpu *Z80Cpu) { panic("Opcode AA unimplemented") },                                      // AA
	func(cpu *Z80Cpu) { panic("Opcode AB unimplemented") },                                      // AB
	func(cpu *Z80Cpu) { panic("Opcode AC unimplemented") },                                      // AC
	func(cpu *Z80Cpu) { panic("Opcode AD unimplemented") },                                      // AD
	func(cpu *Z80Cpu) { panic("Opcode AE unimplemented") },                                      // AE
	func(cpu *Z80Cpu) { panic("Opcode AF unimplemented") },                                      // AF
	func(cpu *Z80Cpu) { panic("Opcode B0 unimplemented") },                                      // B0
	func(cpu *Z80Cpu) { panic("Opcode B1 unimplemented") },                                      // B1
	func(cpu *Z80Cpu) { panic("Opcode B2 unimplemented") },                                      // B2
	func(cpu *Z80Cpu) { panic("Opcode B3 unimplemented") },                                      // B3
	func(cpu *Z80Cpu) { panic("Opcode B4 unimplemented") },                                      // B4
	func(cpu *Z80Cpu) { panic("Opcode B5 unimplemented") },                                      // B5
	func(cpu *Z80Cpu) { panic("Opcode B6 unimplemented") },                                      // B6
	func(cpu *Z80Cpu) { panic("Opcode B7 unimplemented") },                                      // B7
	func(cpu *Z80Cpu) { panic("Opcode B8 unimplemented") },                                      // B8
	func(cpu *Z80Cpu) { panic("Opcode B9 unimplemented") },                                      // B9
	func(cpu *Z80Cpu) { panic("Opcode BA unimplemented") },                                      // BA
	func(cpu *Z80Cpu) { panic("Opcode BB unimplemented") },                                      // BB
	func(cpu *Z80Cpu) { panic("Opcode BC unimplemented") },                                      // BC
	func(cpu *Z80Cpu) { panic("Opcode BD unimplemented") },                                      // BD
	func(cpu *Z80Cpu) { panic("Opcode BE unimplemented") },                                      // BE
	func(cpu *Z80Cpu) { panic("Opcode BF unimplemented") },                                      // BF
	func(cpu *Z80Cpu) { panic("Opcode C0 unimplemented") },                                      // C0
	func(cpu *Z80Cpu) { panic("Opcode C1 unimplemented") },                                      // C1
	func(cpu *Z80Cpu) { panic("Opcode C2 unimplemented") },                                      // C2
	func(cpu *Z80Cpu) { panic("Opcode C3 unimplemented") },                                      // C3
	func(cpu *Z80Cpu) { panic("Opcode C4 unimplemented") },                                      // C4
	func(cpu *Z80Cpu) { panic("Opcode C5 unimplemented") },                                      // C5
	func(cpu *Z80Cpu) { handler_add_R_PC(cpu, &cpu.a) },                                         // C6
	func(cpu *Z80Cpu) { panic("Opcode C7 unimplemented") },                                      // C7
	func(cpu *Z80Cpu) { panic("Opcode C8 unimplemented") },                                      // C8
	func(cpu *Z80Cpu) { panic("Opcode C9 unimplemented") },                                      // C9
	func(cpu *Z80Cpu) { panic("Opcode CA unimplemented") },                                      // CA
	func(cpu *Z80Cpu) { panic("Opcode CB unimplemented") },                                      // CB
	func(cpu *Z80Cpu) { panic("Opcode CC unimplemented") },                                      // CC
	func(cpu *Z80Cpu) { panic("Opcode CD unimplemented") },                                      // CD
	func(cpu *Z80Cpu) { panic("Opcode CE unimplemented") },                                      // CE
	func(cpu *Z80Cpu) { panic("Opcode CF unimplemented") },                                      // CF
	func(cpu *Z80Cpu) { panic("Opcode D0 unimplemented") },                                      // D0
	func(cpu *Z80Cpu) { panic("Opcode D1 unimplemented") },                                      // D1
	func(cpu *Z80Cpu) { panic("Opcode D2 unimplemented") },                                      // D2
	func(cpu *Z80Cpu) { panic("Opcode D3 unimplemented") },                                      // D3
	func(cpu *Z80Cpu) { panic("Opcode D4 unimplemented") },                                      // D4
	func(cpu *Z80Cpu) { panic("Opcode D5 unimplemented") },                                      // D5
	func(cpu *Z80Cpu) { panic("Opcode D6 unimplemented") },                                      // D6
	func(cpu *Z80Cpu) { panic("Opcode D7 unimplemented") },                                      // D7
	func(cpu *Z80Cpu) { panic("Opcode D8 unimplemented") },                                      // D8
	func(cpu *Z80Cpu) { panic("Opcode D9 unimplemented") },                                      // D9
	func(cpu *Z80Cpu) { panic("Opcode DA unimplemented") },                                      // DA
	func(cpu *Z80Cpu) { panic("Opcode DB unimplemented") },                                      // DB
	func(cpu *Z80Cpu) { panic("Opcode DC unimplemented") },                                      // DC
	func(cpu *Z80Cpu) { panic("Opcode DD unimplemented") },                                      // DD
	func(cpu *Z80Cpu) { panic("Opcode DE unimplemented") },                                      // DE
	func(cpu *Z80Cpu) { panic("Opcode DF unimplemented") },                                      // DF
	func(cpu *Z80Cpu) { panic("Opcode E0 unimplemented") },                                      // E0
	func(cpu *Z80Cpu) { panic("Opcode E1 unimplemented") },                                      // E1
	func(cpu *Z80Cpu) { panic("Opcode E2 unimplemented") },                                      // E2
	func(cpu *Z80Cpu) { panic("Opcode E3 unimplemented") },                                      // E3
	func(cpu *Z80Cpu) { panic("Opcode E4 unimplemented") },                                      // E4
	func(cpu *Z80Cpu) { panic("Opcode E5 unimplemented") },                                      // E5
	func(cpu *Z80Cpu) { panic("Opcode E6 unimplemented") },                                      // E6
	func(cpu *Z80Cpu) { panic("Opcode E7 unimplemented") },                                      // E7
	func(cpu *Z80Cpu) { panic("Opcode E8 unimplemented") },                                      // E8
	func(cpu *Z80Cpu) { panic("Opcode E9 unimplemented") },                                      // E9
	func(cpu *Z80Cpu) { panic("Opcode EA unimplemented") },                                      // EA
	func(cpu *Z80Cpu) { panic("Opcode EB unimplemented") },                                      // EB
	func(cpu *Z80Cpu) { panic("Opcode EC unimplemented") },                                      // EC
	func(cpu *Z80Cpu) { panic("Opcode ED unimplemented") },                                      // ED
	func(cpu *Z80Cpu) { panic("Opcode EE unimplemented") },                                      // EE
	func(cpu *Z80Cpu) { panic("Opcode EF unimplemented") },                                      // EF
	func(cpu *Z80Cpu) { panic("Opcode F0 unimplemented") },                                      // F0
	func(cpu *Z80Cpu) { panic("Opcode F1 unimplemented") },                                      // F1
	func(cpu *Z80Cpu) { panic("Opcode F2 unimplemented") },                                      // F2
	func(cpu *Z80Cpu) { panic("Opcode F3 unimplemented") },                                      // F3
	func(cpu *Z80Cpu) { panic("Opcode F4 unimplemented") },                                      // F4
	func(cpu *Z80Cpu) { panic("Opcode F5 unimplemented") },                                      // F5
	func(cpu *Z80Cpu) { panic("Opcode F6 unimplemented") },                                      // F6
	func(cpu *Z80Cpu) { panic("Opcode F7 unimplemented") },                                      // F7
	func(cpu *Z80Cpu) { panic("Opcode F8 unimplemented") },                                      // F8
	func(cpu *Z80Cpu) { panic("Opcode F9 unimplemented") },                                      // F9
	func(cpu *Z80Cpu) { panic("Opcode FA unimplemented") },                                      // FA
	func(cpu *Z80Cpu) { panic("Opcode FB unimplemented") },                                      // FB
	func(cpu *Z80Cpu) { panic("Opcode FC unimplemented") },                                      // FC
	func(cpu *Z80Cpu) { panic("Opcode FD unimplemented") },                                      // FD
	func(cpu *Z80Cpu) { panic("Opcode FE unimplemented") },                                      // FE
	func(cpu *Z80Cpu) { panic("Opcode FF unimplemented") },                                      // FF
}
