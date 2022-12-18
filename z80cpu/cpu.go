package z80cpu

import "fmt"

type Memory interface {
	Read(uint16) uint8
	Write(uint16, uint8)
}

type Z80Interrupt struct {
	Name string
	Mask uint8
	Addr uint16
}

type Z80Cpu struct {
	Mem                 Memory
	a, b, c, d, e, h, l uint8
	sp, pc              uint16

	flagWasZero, flagWasSub, flagHalfCarry, flagCarry bool

	// InterruptEnable and InterruptFlag regs
	IE, IF uint8

	branchWasTaken    bool
	isHalted          bool
	interruptsEnabled bool

	Interrupts []Z80Interrupt

	// Used to hold "OUT" data
	OutBuffer []byte

	// Z80 disassembler, for debugging
	EnableDisas bool
	disas       Z80Disas
}

type Condition uint8

const (
	Condition_C  Condition = 1
	Condition_NC           = 2
	Condition_Z            = 3
	Condition_NZ           = 4
)

func MakeZ80Cpu(mem Memory) *Z80Cpu {
	cpu := &Z80Cpu{
		Mem: mem,
	}
	cpu.Reset()
	return cpu
}

func (cpu *Z80Cpu) Reset() {
	cpu.sp = 0xff
	cpu.pc = 0
	cpu.OutBuffer = make([]byte, 0)
	cpu.isHalted = false
}

func (cpu *Z80Cpu) RegisterInterrupt(interrupt Z80Interrupt) {
	cpu.Interrupts = append(cpu.Interrupts, interrupt)
}

func (cpu *Z80Cpu) SetInterrupt(mask uint8) {
	cpu.IF |= mask
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

func (cpu *Z80Cpu) handleInterrupts() {
	if !cpu.interruptsEnabled {
		return
	}

	interruptValue := cpu.IE & cpu.IF
	if interruptValue == 0 {
		return
	}

	cpu.isHalted = false
	cpu.StackPush16(cpu.pc)
	cpu.interruptsEnabled = false

	interruptWasHandled := false
	for _, interrupt := range cpu.Interrupts {
		if interruptValue&interrupt.Mask != 0 {
			cpu.IF &= ^interrupt.Mask
			cpu.pc = interrupt.Addr
			interruptWasHandled = true
			break
		}
	}

	if !interruptWasHandled {
		panic("Invalid interrupt")
	}
}

func (cpu *Z80Cpu) ExecOne() int {
	cpu.handleInterrupts()

	if cpu.isHalted {
		return 1
	}

	if cpu.EnableDisas {
		_, disas_str := cpu.disas.DisassembleOneFromCPU(cpu)
		fmt.Println(disas_str)
	}

	isCBOpcode := false
	cb_opcode := uint8(0)
	opcode := cpu.fetchOpcode()
	if opcode == 0xcb {
		isCBOpcode = true
		cb_opcode = cpu.Mem.Read(cpu.pc)
	}

	cpu.branchWasTaken = false
	handler := handlers[opcode]
	handler(cpu)

	cycles := 0
	if isCBOpcode {
		cycles += int(cycles_cb[cb_opcode])
	} else if cpu.branchWasTaken {
		cycles += int(cycles_branched[opcode])
	} else {
		cycles += int(cycles_opcode[opcode])
	}

	return cycles
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

func (cpu *Z80Cpu) StackPush16(val uint16) {
	cpu.sp -= 1
	cpu.Mem.Write(cpu.sp, uint8(val>>8))

	cpu.sp -= 1
	cpu.Mem.Write(cpu.sp, uint8(val&0xff))
}

func (cpu *Z80Cpu) StackPop16() uint16 {
	low := cpu.Mem.Read(cpu.sp)
	cpu.sp += 1
	high := cpu.Mem.Read(cpu.sp)
	cpu.sp += 1

	return uint16(high)<<8 | uint16(low)
}

func pack_regcouple(b, a uint8) uint16 {
	return (uint16(b) << 8) | uint16(a)
}

func unpack_regcouple(a uint16) (uint8, uint8) {
	return uint8(a >> 8 & 0xff), uint8(a & 0xff)
}

/*
 *  OPCODE HANDLERS
 */

// LD
func handler_ld_R_16(cpu *Z80Cpu, dst1, dst2 *uint8, value uint16) {
	*dst1, *dst2 = unpack_regcouple(value)
}

func handler_ld_R_16_2(cpu *Z80Cpu, dst *uint16, value uint16) {
	*dst = value
}

func handler_ld_R_8(cpu *Z80Cpu, dst *uint8, value uint8) {
	*dst = value
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

// LDI
func handler_ldi_R_MEM(cpu *Z80Cpu, dst *uint8, addr uint16) {
	*dst = cpu.Mem.Read(addr)
	cpu.h, cpu.l = unpack_regcouple(pack_regcouple(cpu.h, cpu.l) + 1)
}

func handler_ldi_MEM_R(cpu *Z80Cpu, addr uint16, val uint8) {
	cpu.Mem.Write(addr, val)
	cpu.h, cpu.l = unpack_regcouple(pack_regcouple(cpu.h, cpu.l) + 1)
}

// LDD
func handler_ldd_R_MEM(cpu *Z80Cpu, dst *uint8, addr uint16) {
	*dst = cpu.Mem.Read(addr)
	cpu.h, cpu.l = unpack_regcouple(pack_regcouple(cpu.h, cpu.l) - 1)
}

func handler_ldd_MEM_R(cpu *Z80Cpu, addr uint16, val uint8) {
	cpu.Mem.Write(addr, val)
	cpu.h, cpu.l = unpack_regcouple(pack_regcouple(cpu.h, cpu.l) - 1)
}

// INC
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

func handler_inc_R_16_2(cpu *Z80Cpu, dst *uint16) {
	*dst += 1
}

// DEC
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

func handler_dec_R_16_2(cpu *Z80Cpu, dst *uint16) {
	*dst -= 1
}

// ADD
func handler_add_R_8(cpu *Z80Cpu, dst *uint8, src uint8) {
	old_dst := *dst
	res := uint16(old_dst) + uint16(src)
	*dst = uint8(res & 0xff)

	cpu.flagWasZero = *dst == 0
	cpu.flagWasSub = false
	cpu.flagHalfCarry = (old_dst&0xf)+(src&0xf) > 0xf
	cpu.flagCarry = res&0x100 != 0
}

func handler_add_R_16(cpu *Z80Cpu, dst1, dst2 *uint8, src uint16) {
	dst := pack_regcouple(*dst1, *dst2)
	res := dst + src
	*dst1, *dst2 = unpack_regcouple(res)

	cpu.flagWasSub = false
	cpu.flagHalfCarry = ((dst&0xfff)+(src&0xfff) > 0xfff)
	cpu.flagCarry = int(dst)+int(src) > 0xffff
}

// ADC
func handler_adc_R_8(cpu *Z80Cpu, dst *uint8, val uint8) {
	res := *dst
	var carry uint8 = 0
	if cpu.flagCarry {
		carry = 1
	}

	var result uint16 = uint16(res) + uint16(val) + uint16(carry)

	cpu.flagWasZero = uint8(result) == 0
	cpu.flagWasSub = false
	cpu.flagHalfCarry = (*dst&0xf)+(val&0xf)+carry > 0xf
	cpu.flagCarry = result > 0xff

	*dst = uint8(result)
}

// SUB
func handler_sub_R_8(cpu *Z80Cpu, dst *uint8, val uint8) {
	res := *dst - val

	cpu.flagWasZero = res == 0
	cpu.flagWasSub = true
	cpu.flagCarry = *dst < val
	cpu.flagHalfCarry = ((*dst&0xf)-(val&0xf))>>7 != 0

	*dst = res
}

// AND
func handler_and_R_8(cpu *Z80Cpu, dst *uint8, value uint8) {
	*dst = *dst & value

	cpu.flagWasZero = *dst == 0
	cpu.flagHalfCarry = true
	cpu.flagCarry = false
	cpu.flagWasSub = false
}

// XOR
func handler_xor_R_8(cpu *Z80Cpu, dst *uint8, value uint8) {
	*dst = *dst ^ value

	cpu.flagWasZero = *dst == 0
	cpu.flagHalfCarry = false
	cpu.flagCarry = false
	cpu.flagWasSub = false
}

// CP
func handler_cp(cpu *Z80Cpu, v1, v2 uint8) {
	res := v1 - v2

	cpu.flagWasZero = res == 0
	cpu.flagWasSub = true
	cpu.flagCarry = v1 < v2
	cpu.flagHalfCarry = ((v1&0xf)-(v2&0xf))>>7 != 0
}

// CPL
func handler_cpl(cpu *Z80Cpu) {
	cpu.a = ^cpu.a

	cpu.flagWasSub = true
	cpu.flagHalfCarry = true
}

// DAA
func handler_daa(cpu *Z80Cpu) {
	val := cpu.a

	var inc uint8 = 0
	if cpu.flagCarry {
		inc = 0x60
	}

	if cpu.flagHalfCarry || (!cpu.flagWasSub && (val&0xf > 9)) {
		inc |= 0x6
	}
	if cpu.flagCarry || (!cpu.flagWasSub && val > 0x99) {
		inc |= 0x60
	}
	if cpu.flagWasSub {
		val -= inc
	} else {
		val += inc
	}

	if (uint16(inc)<<2)&0x100 != 0 {
		cpu.flagCarry = true
	}

	cpu.flagHalfCarry = false
	cpu.flagWasZero = val == 0

	cpu.a = val
}

// RL
func handler_rlc_R(cpu *Z80Cpu, dst *uint8) {
	carry := *dst >> 7
	res := *dst<<1 | carry

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

func handler_rl_MEM(cpu *Z80Cpu, addr uint16) {
	val := cpu.Mem.Read(addr)
	carry := val >> 7

	val = val << 1
	if cpu.flagCarry {
		val |= 1
	}

	cpu.Mem.Write(addr, val)

	cpu.flagWasZero = val == 0
	cpu.flagWasSub = false
	cpu.flagHalfCarry = false
	cpu.flagCarry = carry != 0
}

// RR
func handler_rrc_R(cpu *Z80Cpu, dst *uint8) {
	carry := *dst & 1
	res := carry | *dst>>1

	cpu.flagWasZero = res == 0
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

// BIT
func handler_bit(cpu *Z80Cpu, bit int, value uint8) {
	cpu.flagWasZero = ((value >> bit) & 1) == 0
	cpu.flagHalfCarry = false
	cpu.flagWasSub = false
}

// RET
func handler_ret(cpu *Z80Cpu) {
	cpu.pc = cpu.StackPop16()
}

// CALL
func handler_call(cpu *Z80Cpu) {
	addr := cpu.getPC16()
	cpu.StackPush16(cpu.pc)
	cpu.pc = addr
}

// JR
func handler_jr(cpu *Z80Cpu, off int8) {
	cpu.pc = uint16(int(cpu.pc) + 1 + int(off))
}

func handler_jr_IF(cpu *Z80Cpu, off int8, cond bool) {
	if cond {
		cpu.pc = uint16(int(cpu.pc) + 1 + int(off))
	} else {
		cpu.pc += 1
	}
}

// OTHER
func handler_out(cpu *Z80Cpu, value uint8) {
	// FIXME: do not ignore the port
	_ = cpu.getPC8()
	cpu.OutBuffer = append(cpu.OutBuffer, value)
}

func handler_halt(cpu *Z80Cpu) {
	cpu.isHalted = true
}

func handler_nop() {}

func handler_stop() {}

var handlers = [256]func(*Z80Cpu){
	func(cpu *Z80Cpu) { handler_nop() },                                                             // 00
	func(cpu *Z80Cpu) { handler_ld_R_16(cpu, &cpu.b, &cpu.c, cpu.getPC16()) },                       // 01
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.b, cpu.c), cpu.a) },                // 02
	func(cpu *Z80Cpu) { handler_inc_R_16(cpu, &cpu.b, &cpu.c) },                                     // 03
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.b) },                                              // 04
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.b) },                                              // 05
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.b, cpu.getPC8()) },                                 // 06
	func(cpu *Z80Cpu) { handler_rlc_R(cpu, &cpu.a); cpu.flagWasZero = false /* rlca */ },            // 07
	func(cpu *Z80Cpu) { handler_ld_MEM_16(cpu, cpu.getPC16(), cpu.sp) },                             // 08
	func(cpu *Z80Cpu) { handler_add_R_16(cpu, &cpu.h, &cpu.l, pack_regcouple(cpu.b, cpu.c)) },       // 09
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.a, pack_regcouple(cpu.b, cpu.c)) },             // 0A
	func(cpu *Z80Cpu) { handler_dec_R_16(cpu, &cpu.b, &cpu.c) },                                     // 0B
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.c) },                                              // 0C
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.c) },                                              // 0D
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.c, cpu.getPC8()) },                                 // 0E
	func(cpu *Z80Cpu) { handler_rrc_R(cpu, &cpu.a); cpu.flagWasZero = false /* rrca */ },            // 0F
	func(cpu *Z80Cpu) { handler_stop() },                                                            // 10
	func(cpu *Z80Cpu) { handler_ld_R_16(cpu, &cpu.d, &cpu.e, cpu.getPC16()) },                       // 11
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.d, cpu.e), cpu.a) },                // 12
	func(cpu *Z80Cpu) { handler_inc_R_16(cpu, &cpu.d, &cpu.e) },                                     // 13
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.d) },                                              // 14
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.d) },                                              // 15
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.d, cpu.getPC8()) },                                 // 16
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.a); cpu.flagWasZero = false /* rla */ },              // 17
	func(cpu *Z80Cpu) { handler_jr(cpu, int8(cpu.Mem.Read(cpu.pc))) },                               // 18
	func(cpu *Z80Cpu) { handler_add_R_16(cpu, &cpu.h, &cpu.l, pack_regcouple(cpu.d, cpu.e)) },       // 19
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.a, pack_regcouple(cpu.d, cpu.e)) },             // 1A
	func(cpu *Z80Cpu) { handler_dec_R_16(cpu, &cpu.d, &cpu.e) },                                     // 1B
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.e) },                                              // 1C
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.e) },                                              // 1D
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.e, cpu.getPC8()) },                                 // 1E
	func(cpu *Z80Cpu) { handler_rr_R(cpu, &cpu.a); cpu.flagWasZero = false /* rra */ },              // 1F
	func(cpu *Z80Cpu) { handler_jr_IF(cpu, int8(cpu.Mem.Read(cpu.pc)), !cpu.flagWasZero) },          // 20
	func(cpu *Z80Cpu) { handler_ld_R_16(cpu, &cpu.h, &cpu.l, cpu.getPC16()) },                       // 21
	func(cpu *Z80Cpu) { handler_ldi_MEM_R(cpu, pack_regcouple(cpu.h, cpu.l), cpu.a) },               // 22
	func(cpu *Z80Cpu) { handler_inc_R_16(cpu, &cpu.h, &cpu.l) },                                     // 23
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.h) },                                              // 24
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.h) },                                              // 25
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.h, cpu.getPC8()) },                                 // 26
	func(cpu *Z80Cpu) { handler_daa(cpu) },                                                          // 27
	func(cpu *Z80Cpu) { handler_jr_IF(cpu, int8(cpu.Mem.Read(cpu.pc)), cpu.flagWasZero) },           // 28
	func(cpu *Z80Cpu) { handler_add_R_16(cpu, &cpu.h, &cpu.l, pack_regcouple(cpu.h, cpu.l)) },       // 29
	func(cpu *Z80Cpu) { handler_ldi_R_MEM(cpu, &cpu.a, pack_regcouple(cpu.h, cpu.l)) },              // 2A
	func(cpu *Z80Cpu) { handler_dec_R_16(cpu, &cpu.h, &cpu.l) },                                     // 2B
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.l) },                                              // 2C
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.l) },                                              // 2D
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.l, cpu.getPC8()) },                                 // 2E
	func(cpu *Z80Cpu) { handler_cpl(cpu) },                                                          // 2F
	func(cpu *Z80Cpu) { handler_jr_IF(cpu, int8(cpu.Mem.Read(cpu.pc)), !cpu.flagCarry) },            // 30
	func(cpu *Z80Cpu) { handler_ld_R_16_2(cpu, &cpu.sp, cpu.getPC16()) },                            // 31
	func(cpu *Z80Cpu) { handler_ldd_MEM_R(cpu, pack_regcouple(cpu.h, cpu.l), cpu.a) },               // 32
	func(cpu *Z80Cpu) { handler_inc_R_16_2(cpu, &cpu.sp) },                                          // 33
	func(cpu *Z80Cpu) { panic("Opcode 34 unimplemented") },                                          // 34
	func(cpu *Z80Cpu) { panic("Opcode 35 unimplemented") },                                          // 35
	func(cpu *Z80Cpu) { panic("Opcode 36 unimplemented") },                                          // 36
	func(cpu *Z80Cpu) { panic("Opcode 37 unimplemented") },                                          // 37
	func(cpu *Z80Cpu) { handler_jr_IF(cpu, int8(cpu.Mem.Read(cpu.pc)), cpu.flagCarry) },             // 38
	func(cpu *Z80Cpu) { handler_add_R_16(cpu, &cpu.h, &cpu.l, cpu.sp) },                             // 39
	func(cpu *Z80Cpu) { handler_ldd_R_MEM(cpu, &cpu.a, pack_regcouple(cpu.h, cpu.l)) },              // 3A
	func(cpu *Z80Cpu) { handler_dec_R_16_2(cpu, &cpu.sp) },                                          // 3B
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.a) },                                              // 3C
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.a) },                                              // 3D
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.a, cpu.getPC8()) },                                 // 3E
	func(cpu *Z80Cpu) { panic("Opcode 3F unimplemented") },                                          // 3F
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.b, cpu.b) },                                        // 40
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.b, cpu.c) },                                        // 41
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.b, cpu.d) },                                        // 42
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.b, cpu.e) },                                        // 43
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.b, cpu.h) },                                        // 44
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.b, cpu.l) },                                        // 45
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.b, pack_regcouple(cpu.h, cpu.l)) },             // 46
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.b, cpu.a) },                                        // 47
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.c, cpu.b) },                                        // 48
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.c, cpu.c) },                                        // 49
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.c, cpu.d) },                                        // 4A
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.c, cpu.e) },                                        // 4B
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.c, cpu.h) },                                        // 4C
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.c, cpu.l) },                                        // 4D
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.c, pack_regcouple(cpu.h, cpu.l)) },             // 4E
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.c, cpu.a) },                                        // 4F
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.d, cpu.b) },                                        // 50
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.d, cpu.c) },                                        // 51
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.d, cpu.d) },                                        // 52
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.d, cpu.e) },                                        // 53
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.d, cpu.h) },                                        // 54
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.d, cpu.l) },                                        // 55
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.d, pack_regcouple(cpu.h, cpu.l)) },             // 56
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.d, cpu.a) },                                        // 57
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.e, cpu.b) },                                        // 58
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.e, cpu.c) },                                        // 59
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.e, cpu.d) },                                        // 5A
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.e, cpu.e) },                                        // 5B
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.e, cpu.h) },                                        // 5C
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.e, cpu.l) },                                        // 5D
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.e, pack_regcouple(cpu.h, cpu.l)) },             // 5E
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.e, cpu.a) },                                        // 5F
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.h, cpu.b) },                                        // 60
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.h, cpu.c) },                                        // 61
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.h, cpu.d) },                                        // 62
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.h, cpu.e) },                                        // 63
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.h, cpu.h) },                                        // 64
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.h, cpu.l) },                                        // 65
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.h, pack_regcouple(cpu.h, cpu.l)) },             // 66
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.h, cpu.a) },                                        // 67
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.l, cpu.b) },                                        // 68
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.l, cpu.c) },                                        // 69
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.l, cpu.d) },                                        // 6A
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.l, cpu.e) },                                        // 6B
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.l, cpu.h) },                                        // 6C
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.l, cpu.l) },                                        // 6D
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.l, pack_regcouple(cpu.h, cpu.l)) },             // 6E
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.l, cpu.a) },                                        // 6F
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.h, cpu.l), cpu.b) },                // 70
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.h, cpu.l), cpu.c) },                // 71
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.h, cpu.l), cpu.d) },                // 72
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.h, cpu.l), cpu.e) },                // 73
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.h, cpu.l), cpu.h) },                // 74
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.h, cpu.l), cpu.l) },                // 75
	func(cpu *Z80Cpu) { handler_halt(cpu) },                                                         // 76
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.h, cpu.l), cpu.a) },                // 77
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.a, cpu.b) },                                        // 78
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.a, cpu.c) },                                        // 79
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.a, cpu.d) },                                        // 7A
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.a, cpu.e) },                                        // 7B
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.a, cpu.h) },                                        // 7C
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.a, cpu.l) },                                        // 7D
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.a, pack_regcouple(cpu.h, cpu.l)) },             // 7E
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.a, cpu.a) },                                        // 7F
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.a, cpu.b) },                                       // 80
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.a, cpu.c) },                                       // 81
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.a, cpu.d) },                                       // 82
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.a, cpu.e) },                                       // 83
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.a, cpu.h) },                                       // 84
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.a, cpu.l) },                                       // 85
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.a, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) },  // 86
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.a, cpu.a) },                                       // 87
	func(cpu *Z80Cpu) { panic("Opcode 88 unimplemented") },                                          // 88
	func(cpu *Z80Cpu) { panic("Opcode 89 unimplemented") },                                          // 89
	func(cpu *Z80Cpu) { panic("Opcode 8A unimplemented") },                                          // 8A
	func(cpu *Z80Cpu) { panic("Opcode 8B unimplemented") },                                          // 8B
	func(cpu *Z80Cpu) { panic("Opcode 8C unimplemented") },                                          // 8C
	func(cpu *Z80Cpu) { panic("Opcode 8D unimplemented") },                                          // 8D
	func(cpu *Z80Cpu) { panic("Opcode 8E unimplemented") },                                          // 8E
	func(cpu *Z80Cpu) { panic("Opcode 8F unimplemented") },                                          // 8F
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.a, cpu.b) },                                       // 90
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.a, cpu.c) },                                       // 91
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.a, cpu.d) },                                       // 92
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.a, cpu.e) },                                       // 93
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.a, cpu.h) },                                       // 94
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.a, cpu.l) },                                       // 95
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.a, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) },  // 96
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.a, cpu.a) },                                       // 97
	func(cpu *Z80Cpu) { panic("Opcode 98 unimplemented") },                                          // 98
	func(cpu *Z80Cpu) { panic("Opcode 99 unimplemented") },                                          // 99
	func(cpu *Z80Cpu) { panic("Opcode 9A unimplemented") },                                          // 9A
	func(cpu *Z80Cpu) { panic("Opcode 9B unimplemented") },                                          // 9B
	func(cpu *Z80Cpu) { panic("Opcode 9C unimplemented") },                                          // 9C
	func(cpu *Z80Cpu) { panic("Opcode 9D unimplemented") },                                          // 9D
	func(cpu *Z80Cpu) { panic("Opcode 9E unimplemented") },                                          // 9E
	func(cpu *Z80Cpu) { panic("Opcode 9F unimplemented") },                                          // 9F
	func(cpu *Z80Cpu) { panic("Opcode A0 unimplemented") },                                          // A0
	func(cpu *Z80Cpu) { panic("Opcode A1 unimplemented") },                                          // A1
	func(cpu *Z80Cpu) { panic("Opcode A2 unimplemented") },                                          // A2
	func(cpu *Z80Cpu) { panic("Opcode A3 unimplemented") },                                          // A3
	func(cpu *Z80Cpu) { panic("Opcode A4 unimplemented") },                                          // A4
	func(cpu *Z80Cpu) { panic("Opcode A5 unimplemented") },                                          // A5
	func(cpu *Z80Cpu) { panic("Opcode A6 unimplemented") },                                          // A6
	func(cpu *Z80Cpu) { panic("Opcode A7 unimplemented") },                                          // A7
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.a, cpu.b) },                                       // A8
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.a, cpu.c) },                                       // A9
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.a, cpu.d) },                                       // AA
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.a, cpu.e) },                                       // AB
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.a, cpu.h) },                                       // AC
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.a, cpu.l) },                                       // AD
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.a, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) },  // AE
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.a, cpu.a) },                                       // AF
	func(cpu *Z80Cpu) { panic("Opcode B0 unimplemented") },                                          // B0
	func(cpu *Z80Cpu) { panic("Opcode B1 unimplemented") },                                          // B1
	func(cpu *Z80Cpu) { panic("Opcode B2 unimplemented") },                                          // B2
	func(cpu *Z80Cpu) { panic("Opcode B3 unimplemented") },                                          // B3
	func(cpu *Z80Cpu) { panic("Opcode B4 unimplemented") },                                          // B4
	func(cpu *Z80Cpu) { panic("Opcode B5 unimplemented") },                                          // B5
	func(cpu *Z80Cpu) { panic("Opcode B6 unimplemented") },                                          // B6
	func(cpu *Z80Cpu) { panic("Opcode B7 unimplemented") },                                          // B7
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.a, cpu.b) },                                             // B8
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.a, cpu.c) },                                             // B9
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.a, cpu.d) },                                             // BA
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.a, cpu.e) },                                             // BB
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.a, cpu.h) },                                             // BC
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.a, cpu.l) },                                             // BD
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.a, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) },        // BE
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.a, cpu.a) },                                             // BF
	func(cpu *Z80Cpu) { panic("Opcode C0 unimplemented") },                                          // C0
	func(cpu *Z80Cpu) { cpu.b, cpu.c = unpack_regcouple(cpu.StackPop16()) },                         // C1
	func(cpu *Z80Cpu) { panic("Opcode C2 unimplemented") },                                          // C2
	func(cpu *Z80Cpu) { panic("Opcode C3 unimplemented") },                                          // C3
	func(cpu *Z80Cpu) { panic("Opcode C4 unimplemented") },                                          // C4
	func(cpu *Z80Cpu) { cpu.StackPush16(pack_regcouple(cpu.b, cpu.c)) },                             // C5
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.a, cpu.getPC8()) },                                // C6
	func(cpu *Z80Cpu) { panic("Opcode C7 unimplemented") },                                          // C7
	func(cpu *Z80Cpu) { panic("Opcode C8 unimplemented") },                                          // C8
	func(cpu *Z80Cpu) { handler_ret(cpu) },                                                          // C9
	func(cpu *Z80Cpu) { panic("Opcode CA unimplemented") },                                          // CA
	func(cpu *Z80Cpu) { cb_handlers[cpu.getPC8()](cpu) },                                            // CB
	func(cpu *Z80Cpu) { panic("Opcode CC unimplemented") },                                          // CC
	func(cpu *Z80Cpu) { handler_call(cpu) },                                                         // CD
	func(cpu *Z80Cpu) { handler_adc_R_8(cpu, &cpu.a, cpu.getPC8()) },                                // CE
	func(cpu *Z80Cpu) { panic("Opcode CF unimplemented") },                                          // CF
	func(cpu *Z80Cpu) { panic("Opcode D0 unimplemented") },                                          // D0
	func(cpu *Z80Cpu) { cpu.d, cpu.e = unpack_regcouple(cpu.StackPop16()) },                         // D1
	func(cpu *Z80Cpu) { panic("Opcode D2 unimplemented") },                                          // D2
	func(cpu *Z80Cpu) { handler_out(cpu, cpu.a) },                                                   // D3
	func(cpu *Z80Cpu) { panic("Opcode D4 unimplemented") },                                          // D4
	func(cpu *Z80Cpu) { cpu.StackPush16(pack_regcouple(cpu.d, cpu.e)) },                             // D5
	func(cpu *Z80Cpu) { panic("Opcode D6 unimplemented") },                                          // D6
	func(cpu *Z80Cpu) { panic("Opcode D7 unimplemented") },                                          // D7
	func(cpu *Z80Cpu) { panic("Opcode D8 unimplemented") },                                          // D8
	func(cpu *Z80Cpu) { panic("Opcode D9 unimplemented") },                                          // D9
	func(cpu *Z80Cpu) { panic("Opcode DA unimplemented") },                                          // DA
	func(cpu *Z80Cpu) { panic("Opcode DB unimplemented") },                                          // DB
	func(cpu *Z80Cpu) { panic("Opcode DC unimplemented") },                                          // DC
	func(cpu *Z80Cpu) { panic("Opcode DD unimplemented") },                                          // DD
	func(cpu *Z80Cpu) { panic("Opcode DE unimplemented") },                                          // DE
	func(cpu *Z80Cpu) { panic("Opcode DF unimplemented") },                                          // DF
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, uint16(cpu.getPC8())+0xFF00, cpu.a) },                 // E0
	func(cpu *Z80Cpu) { cpu.h, cpu.l = unpack_regcouple(cpu.StackPop16()) },                         // E1
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, uint16(cpu.c)+0xFF00, cpu.a) },                        // E2
	func(cpu *Z80Cpu) { panic("Opcode E3 unimplemented") },                                          // E3
	func(cpu *Z80Cpu) { panic("Opcode E4 unimplemented") },                                          // E4
	func(cpu *Z80Cpu) { cpu.StackPush16(pack_regcouple(cpu.h, cpu.l)) },                             // E5
	func(cpu *Z80Cpu) { handler_and_R_8(cpu, &cpu.a, cpu.getPC8()) },                                // E6
	func(cpu *Z80Cpu) { panic("Opcode E7 unimplemented") },                                          // E7
	func(cpu *Z80Cpu) { panic("Opcode E8 unimplemented") },                                          // E8
	func(cpu *Z80Cpu) { panic("Opcode E9 unimplemented") },                                          // E9
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, cpu.getPC16(), cpu.a) },                               // EA
	func(cpu *Z80Cpu) { panic("Opcode EB unimplemented") },                                          // EB
	func(cpu *Z80Cpu) { panic("Opcode EC unimplemented") },                                          // EC
	func(cpu *Z80Cpu) { panic("Opcode ED unimplemented") },                                          // ED
	func(cpu *Z80Cpu) { panic("Opcode EE unimplemented") },                                          // EE
	func(cpu *Z80Cpu) { panic("Opcode EF unimplemented") },                                          // EF
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.a, uint16(cpu.getPC8())+0xFF00) },              // F0
	func(cpu *Z80Cpu) { a, f := unpack_regcouple(cpu.StackPop16()); cpu.a = a; cpu.UnpackFlags(f) }, // F1
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.a, uint16(cpu.c)+0xFF00) },                     // F2
	func(cpu *Z80Cpu) { panic("Opcode F3 unimplemented") },                                          // F3
	func(cpu *Z80Cpu) { panic("Opcode F4 unimplemented") },                                          // F4
	func(cpu *Z80Cpu) { cpu.StackPush16(pack_regcouple(cpu.a, cpu.PackFlags())) },                   // F5
	func(cpu *Z80Cpu) { panic("Opcode F6 unimplemented") },                                          // F6
	func(cpu *Z80Cpu) { panic("Opcode F7 unimplemented") },                                          // F7
	func(cpu *Z80Cpu) { panic("Opcode F8 unimplemented") },                                          // F8
	func(cpu *Z80Cpu) { panic("Opcode F9 unimplemented") },                                          // F9
	func(cpu *Z80Cpu) { panic("Opcode FA unimplemented") },                                          // FA
	func(cpu *Z80Cpu) { panic("Opcode FB unimplemented") },                                          // FB
	func(cpu *Z80Cpu) { panic("Opcode FC unimplemented") },                                          // FC
	func(cpu *Z80Cpu) { panic("Opcode FD unimplemented") },                                          // FD
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.a, cpu.getPC8()) },                                      // FE
	func(cpu *Z80Cpu) { panic("Opcode FF unimplemented") },                                          // FF
}

var cb_handlers = [256]func(*Z80Cpu){
	func(cpu *Z80Cpu) { panic("CB Opcode 00 unimplemented") },                             // 00
	func(cpu *Z80Cpu) { panic("CB Opcode 01 unimplemented") },                             // 01
	func(cpu *Z80Cpu) { panic("CB Opcode 02 unimplemented") },                             // 02
	func(cpu *Z80Cpu) { panic("CB Opcode 03 unimplemented") },                             // 03
	func(cpu *Z80Cpu) { panic("CB Opcode 04 unimplemented") },                             // 04
	func(cpu *Z80Cpu) { panic("CB Opcode 05 unimplemented") },                             // 05
	func(cpu *Z80Cpu) { panic("CB Opcode 06 unimplemented") },                             // 06
	func(cpu *Z80Cpu) { panic("CB Opcode 07 unimplemented") },                             // 07
	func(cpu *Z80Cpu) { panic("CB Opcode 08 unimplemented") },                             // 08
	func(cpu *Z80Cpu) { panic("CB Opcode 09 unimplemented") },                             // 09
	func(cpu *Z80Cpu) { panic("CB Opcode 0A unimplemented") },                             // 0A
	func(cpu *Z80Cpu) { panic("CB Opcode 0B unimplemented") },                             // 0B
	func(cpu *Z80Cpu) { panic("CB Opcode 0C unimplemented") },                             // 0C
	func(cpu *Z80Cpu) { panic("CB Opcode 0D unimplemented") },                             // 0D
	func(cpu *Z80Cpu) { panic("CB Opcode 0E unimplemented") },                             // 0E
	func(cpu *Z80Cpu) { panic("CB Opcode 0F unimplemented") },                             // 0F
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.b) },                                       // 10
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.c) },                                       // 11
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.d) },                                       // 12
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.e) },                                       // 13
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.h) },                                       // 14
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.l) },                                       // 15
	func(cpu *Z80Cpu) { handler_rl_MEM(cpu, pack_regcouple(cpu.h, cpu.l)) },               // 16
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.a) },                                       // 17
	func(cpu *Z80Cpu) { panic("CB Opcode 18 unimplemented") },                             // 18
	func(cpu *Z80Cpu) { panic("CB Opcode 19 unimplemented") },                             // 19
	func(cpu *Z80Cpu) { panic("CB Opcode 1A unimplemented") },                             // 1A
	func(cpu *Z80Cpu) { panic("CB Opcode 1B unimplemented") },                             // 1B
	func(cpu *Z80Cpu) { panic("CB Opcode 1C unimplemented") },                             // 1C
	func(cpu *Z80Cpu) { panic("CB Opcode 1D unimplemented") },                             // 1D
	func(cpu *Z80Cpu) { panic("CB Opcode 1E unimplemented") },                             // 1E
	func(cpu *Z80Cpu) { panic("CB Opcode 1F unimplemented") },                             // 1F
	func(cpu *Z80Cpu) { panic("CB Opcode 20 unimplemented") },                             // 20
	func(cpu *Z80Cpu) { panic("CB Opcode 21 unimplemented") },                             // 21
	func(cpu *Z80Cpu) { panic("CB Opcode 22 unimplemented") },                             // 22
	func(cpu *Z80Cpu) { panic("CB Opcode 23 unimplemented") },                             // 23
	func(cpu *Z80Cpu) { panic("CB Opcode 24 unimplemented") },                             // 24
	func(cpu *Z80Cpu) { panic("CB Opcode 25 unimplemented") },                             // 25
	func(cpu *Z80Cpu) { panic("CB Opcode 26 unimplemented") },                             // 26
	func(cpu *Z80Cpu) { panic("CB Opcode 27 unimplemented") },                             // 27
	func(cpu *Z80Cpu) { panic("CB Opcode 28 unimplemented") },                             // 28
	func(cpu *Z80Cpu) { panic("CB Opcode 29 unimplemented") },                             // 29
	func(cpu *Z80Cpu) { panic("CB Opcode 2A unimplemented") },                             // 2A
	func(cpu *Z80Cpu) { panic("CB Opcode 2B unimplemented") },                             // 2B
	func(cpu *Z80Cpu) { panic("CB Opcode 2C unimplemented") },                             // 2C
	func(cpu *Z80Cpu) { panic("CB Opcode 2D unimplemented") },                             // 2D
	func(cpu *Z80Cpu) { panic("CB Opcode 2E unimplemented") },                             // 2E
	func(cpu *Z80Cpu) { panic("CB Opcode 2F unimplemented") },                             // 2F
	func(cpu *Z80Cpu) { panic("CB Opcode 30 unimplemented") },                             // 30
	func(cpu *Z80Cpu) { panic("CB Opcode 31 unimplemented") },                             // 31
	func(cpu *Z80Cpu) { panic("CB Opcode 32 unimplemented") },                             // 32
	func(cpu *Z80Cpu) { panic("CB Opcode 33 unimplemented") },                             // 33
	func(cpu *Z80Cpu) { panic("CB Opcode 34 unimplemented") },                             // 34
	func(cpu *Z80Cpu) { panic("CB Opcode 35 unimplemented") },                             // 35
	func(cpu *Z80Cpu) { panic("CB Opcode 36 unimplemented") },                             // 36
	func(cpu *Z80Cpu) { panic("CB Opcode 37 unimplemented") },                             // 37
	func(cpu *Z80Cpu) { panic("CB Opcode 38 unimplemented") },                             // 38
	func(cpu *Z80Cpu) { panic("CB Opcode 39 unimplemented") },                             // 39
	func(cpu *Z80Cpu) { panic("CB Opcode 3A unimplemented") },                             // 3A
	func(cpu *Z80Cpu) { panic("CB Opcode 3B unimplemented") },                             // 3B
	func(cpu *Z80Cpu) { panic("CB Opcode 3C unimplemented") },                             // 3C
	func(cpu *Z80Cpu) { panic("CB Opcode 3D unimplemented") },                             // 3D
	func(cpu *Z80Cpu) { panic("CB Opcode 3E unimplemented") },                             // 3E
	func(cpu *Z80Cpu) { panic("CB Opcode 3F unimplemented") },                             // 3F
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.b) },                                      // 40
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.c) },                                      // 41
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.d) },                                      // 42
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.e) },                                      // 43
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.h) },                                      // 44
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.l) },                                      // 45
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) }, // 46
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.a) },                                      // 47
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.b) },                                      // 48
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.c) },                                      // 49
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.d) },                                      // 4A
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.e) },                                      // 4B
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.h) },                                      // 4C
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.l) },                                      // 4D
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) }, // 4E
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.a) },                                      // 4F
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.b) },                                      // 50
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.c) },                                      // 51
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.d) },                                      // 52
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.e) },                                      // 53
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.h) },                                      // 54
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.l) },                                      // 55
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) }, // 56
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.a) },                                      // 57
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.b) },                                      // 58
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.c) },                                      // 59
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.d) },                                      // 5A
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.e) },                                      // 5B
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.h) },                                      // 5C
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.l) },                                      // 5D
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) }, // 5E
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.a) },                                      // 5F
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.b) },                                      // 60
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.c) },                                      // 61
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.d) },                                      // 62
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.e) },                                      // 63
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.h) },                                      // 64
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.l) },                                      // 65
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) }, // 66
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.a) },                                      // 67
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.b) },                                      // 68
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.c) },                                      // 69
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.d) },                                      // 6A
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.e) },                                      // 6B
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.h) },                                      // 6C
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.l) },                                      // 6D
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) }, // 6E
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.a) },                                      // 6F
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.b) },                                      // 70
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.c) },                                      // 71
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.d) },                                      // 72
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.e) },                                      // 73
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.h) },                                      // 74
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.l) },                                      // 75
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) }, // 76
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.a) },                                      // 77
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.b) },                                      // 78
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.c) },                                      // 79
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.d) },                                      // 7A
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.e) },                                      // 7B
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.h) },                                      // 7C
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.l) },                                      // 7D
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.Mem.Read(pack_regcouple(cpu.h, cpu.l))) }, // 7E
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.a) },                                      // 7F
	func(cpu *Z80Cpu) { panic("CB Opcode 80 unimplemented") },                             // 80
	func(cpu *Z80Cpu) { panic("CB Opcode 81 unimplemented") },                             // 81
	func(cpu *Z80Cpu) { panic("CB Opcode 82 unimplemented") },                             // 82
	func(cpu *Z80Cpu) { panic("CB Opcode 83 unimplemented") },                             // 83
	func(cpu *Z80Cpu) { panic("CB Opcode 84 unimplemented") },                             // 84
	func(cpu *Z80Cpu) { panic("CB Opcode 85 unimplemented") },                             // 85
	func(cpu *Z80Cpu) { panic("CB Opcode 86 unimplemented") },                             // 86
	func(cpu *Z80Cpu) { panic("CB Opcode 87 unimplemented") },                             // 87
	func(cpu *Z80Cpu) { panic("CB Opcode 88 unimplemented") },                             // 88
	func(cpu *Z80Cpu) { panic("CB Opcode 89 unimplemented") },                             // 89
	func(cpu *Z80Cpu) { panic("CB Opcode 8A unimplemented") },                             // 8A
	func(cpu *Z80Cpu) { panic("CB Opcode 8B unimplemented") },                             // 8B
	func(cpu *Z80Cpu) { panic("CB Opcode 8C unimplemented") },                             // 8C
	func(cpu *Z80Cpu) { panic("CB Opcode 8D unimplemented") },                             // 8D
	func(cpu *Z80Cpu) { panic("CB Opcode 8E unimplemented") },                             // 8E
	func(cpu *Z80Cpu) { panic("CB Opcode 8F unimplemented") },                             // 8F
	func(cpu *Z80Cpu) { panic("CB Opcode 90 unimplemented") },                             // 90
	func(cpu *Z80Cpu) { panic("CB Opcode 91 unimplemented") },                             // 91
	func(cpu *Z80Cpu) { panic("CB Opcode 92 unimplemented") },                             // 92
	func(cpu *Z80Cpu) { panic("CB Opcode 93 unimplemented") },                             // 93
	func(cpu *Z80Cpu) { panic("CB Opcode 94 unimplemented") },                             // 94
	func(cpu *Z80Cpu) { panic("CB Opcode 95 unimplemented") },                             // 95
	func(cpu *Z80Cpu) { panic("CB Opcode 96 unimplemented") },                             // 96
	func(cpu *Z80Cpu) { panic("CB Opcode 97 unimplemented") },                             // 97
	func(cpu *Z80Cpu) { panic("CB Opcode 98 unimplemented") },                             // 98
	func(cpu *Z80Cpu) { panic("CB Opcode 99 unimplemented") },                             // 99
	func(cpu *Z80Cpu) { panic("CB Opcode 9A unimplemented") },                             // 9A
	func(cpu *Z80Cpu) { panic("CB Opcode 9B unimplemented") },                             // 9B
	func(cpu *Z80Cpu) { panic("CB Opcode 9C unimplemented") },                             // 9C
	func(cpu *Z80Cpu) { panic("CB Opcode 9D unimplemented") },                             // 9D
	func(cpu *Z80Cpu) { panic("CB Opcode 9E unimplemented") },                             // 9E
	func(cpu *Z80Cpu) { panic("CB Opcode 9F unimplemented") },                             // 9F
	func(cpu *Z80Cpu) { panic("CB Opcode A0 unimplemented") },                             // A0
	func(cpu *Z80Cpu) { panic("CB Opcode A1 unimplemented") },                             // A1
	func(cpu *Z80Cpu) { panic("CB Opcode A2 unimplemented") },                             // A2
	func(cpu *Z80Cpu) { panic("CB Opcode A3 unimplemented") },                             // A3
	func(cpu *Z80Cpu) { panic("CB Opcode A4 unimplemented") },                             // A4
	func(cpu *Z80Cpu) { panic("CB Opcode A5 unimplemented") },                             // A5
	func(cpu *Z80Cpu) { panic("CB Opcode A6 unimplemented") },                             // A6
	func(cpu *Z80Cpu) { panic("CB Opcode A7 unimplemented") },                             // A7
	func(cpu *Z80Cpu) { panic("CB Opcode A8 unimplemented") },                             // A8
	func(cpu *Z80Cpu) { panic("CB Opcode A9 unimplemented") },                             // A9
	func(cpu *Z80Cpu) { panic("CB Opcode AA unimplemented") },                             // AA
	func(cpu *Z80Cpu) { panic("CB Opcode AB unimplemented") },                             // AB
	func(cpu *Z80Cpu) { panic("CB Opcode AC unimplemented") },                             // AC
	func(cpu *Z80Cpu) { panic("CB Opcode AD unimplemented") },                             // AD
	func(cpu *Z80Cpu) { panic("CB Opcode AE unimplemented") },                             // AE
	func(cpu *Z80Cpu) { panic("CB Opcode AF unimplemented") },                             // AF
	func(cpu *Z80Cpu) { panic("CB Opcode B0 unimplemented") },                             // B0
	func(cpu *Z80Cpu) { panic("CB Opcode B1 unimplemented") },                             // B1
	func(cpu *Z80Cpu) { panic("CB Opcode B2 unimplemented") },                             // B2
	func(cpu *Z80Cpu) { panic("CB Opcode B3 unimplemented") },                             // B3
	func(cpu *Z80Cpu) { panic("CB Opcode B4 unimplemented") },                             // B4
	func(cpu *Z80Cpu) { panic("CB Opcode B5 unimplemented") },                             // B5
	func(cpu *Z80Cpu) { panic("CB Opcode B6 unimplemented") },                             // B6
	func(cpu *Z80Cpu) { panic("CB Opcode B7 unimplemented") },                             // B7
	func(cpu *Z80Cpu) { panic("CB Opcode B8 unimplemented") },                             // B8
	func(cpu *Z80Cpu) { panic("CB Opcode B9 unimplemented") },                             // B9
	func(cpu *Z80Cpu) { panic("CB Opcode BA unimplemented") },                             // BA
	func(cpu *Z80Cpu) { panic("CB Opcode BB unimplemented") },                             // BB
	func(cpu *Z80Cpu) { panic("CB Opcode BC unimplemented") },                             // BC
	func(cpu *Z80Cpu) { panic("CB Opcode BD unimplemented") },                             // BD
	func(cpu *Z80Cpu) { panic("CB Opcode BE unimplemented") },                             // BE
	func(cpu *Z80Cpu) { panic("CB Opcode BF unimplemented") },                             // BF
	func(cpu *Z80Cpu) { panic("CB Opcode C0 unimplemented") },                             // C0
	func(cpu *Z80Cpu) { panic("CB Opcode C1 unimplemented") },                             // C1
	func(cpu *Z80Cpu) { panic("CB Opcode C2 unimplemented") },                             // C2
	func(cpu *Z80Cpu) { panic("CB Opcode C3 unimplemented") },                             // C3
	func(cpu *Z80Cpu) { panic("CB Opcode C4 unimplemented") },                             // C4
	func(cpu *Z80Cpu) { panic("CB Opcode C5 unimplemented") },                             // C5
	func(cpu *Z80Cpu) { panic("CB Opcode C6 unimplemented") },                             // C6
	func(cpu *Z80Cpu) { panic("CB Opcode C7 unimplemented") },                             // C7
	func(cpu *Z80Cpu) { panic("CB Opcode C8 unimplemented") },                             // C8
	func(cpu *Z80Cpu) { panic("CB Opcode C9 unimplemented") },                             // C9
	func(cpu *Z80Cpu) { panic("CB Opcode CA unimplemented") },                             // CA
	func(cpu *Z80Cpu) { panic("CB Opcode CB unimplemented") },                             // CB
	func(cpu *Z80Cpu) { panic("CB Opcode CC unimplemented") },                             // CC
	func(cpu *Z80Cpu) { panic("CB Opcode CD unimplemented") },                             // CD
	func(cpu *Z80Cpu) { panic("CB Opcode CE unimplemented") },                             // CE
	func(cpu *Z80Cpu) { panic("CB Opcode CF unimplemented") },                             // CF
	func(cpu *Z80Cpu) { panic("CB Opcode D0 unimplemented") },                             // D0
	func(cpu *Z80Cpu) { panic("CB Opcode D1 unimplemented") },                             // D1
	func(cpu *Z80Cpu) { panic("CB Opcode D2 unimplemented") },                             // D2
	func(cpu *Z80Cpu) { panic("CB Opcode D3 unimplemented") },                             // D3
	func(cpu *Z80Cpu) { panic("CB Opcode D4 unimplemented") },                             // D4
	func(cpu *Z80Cpu) { panic("CB Opcode D5 unimplemented") },                             // D5
	func(cpu *Z80Cpu) { panic("CB Opcode D6 unimplemented") },                             // D6
	func(cpu *Z80Cpu) { panic("CB Opcode D7 unimplemented") },                             // D7
	func(cpu *Z80Cpu) { panic("CB Opcode D8 unimplemented") },                             // D8
	func(cpu *Z80Cpu) { panic("CB Opcode D9 unimplemented") },                             // D9
	func(cpu *Z80Cpu) { panic("CB Opcode DA unimplemented") },                             // DA
	func(cpu *Z80Cpu) { panic("CB Opcode DB unimplemented") },                             // DB
	func(cpu *Z80Cpu) { panic("CB Opcode DC unimplemented") },                             // DC
	func(cpu *Z80Cpu) { panic("CB Opcode DD unimplemented") },                             // DD
	func(cpu *Z80Cpu) { panic("CB Opcode DE unimplemented") },                             // DE
	func(cpu *Z80Cpu) { panic("CB Opcode DF unimplemented") },                             // DF
	func(cpu *Z80Cpu) { panic("CB Opcode E0 unimplemented") },                             // E0
	func(cpu *Z80Cpu) { panic("CB Opcode E1 unimplemented") },                             // E1
	func(cpu *Z80Cpu) { panic("CB Opcode E2 unimplemented") },                             // E2
	func(cpu *Z80Cpu) { panic("CB Opcode E3 unimplemented") },                             // E3
	func(cpu *Z80Cpu) { panic("CB Opcode E4 unimplemented") },                             // E4
	func(cpu *Z80Cpu) { panic("CB Opcode E5 unimplemented") },                             // E5
	func(cpu *Z80Cpu) { panic("CB Opcode E6 unimplemented") },                             // E6
	func(cpu *Z80Cpu) { panic("CB Opcode E7 unimplemented") },                             // E7
	func(cpu *Z80Cpu) { panic("CB Opcode E8 unimplemented") },                             // E8
	func(cpu *Z80Cpu) { panic("CB Opcode E9 unimplemented") },                             // E9
	func(cpu *Z80Cpu) { panic("CB Opcode EA unimplemented") },                             // EA
	func(cpu *Z80Cpu) { panic("CB Opcode EB unimplemented") },                             // EB
	func(cpu *Z80Cpu) { panic("CB Opcode EC unimplemented") },                             // EC
	func(cpu *Z80Cpu) { panic("CB Opcode ED unimplemented") },                             // ED
	func(cpu *Z80Cpu) { panic("CB Opcode EE unimplemented") },                             // EE
	func(cpu *Z80Cpu) { panic("CB Opcode EF unimplemented") },                             // EF
	func(cpu *Z80Cpu) { panic("CB Opcode F0 unimplemented") },                             // F0
	func(cpu *Z80Cpu) { panic("CB Opcode F1 unimplemented") },                             // F1
	func(cpu *Z80Cpu) { panic("CB Opcode F2 unimplemented") },                             // F2
	func(cpu *Z80Cpu) { panic("CB Opcode F3 unimplemented") },                             // F3
	func(cpu *Z80Cpu) { panic("CB Opcode F4 unimplemented") },                             // F4
	func(cpu *Z80Cpu) { panic("CB Opcode F5 unimplemented") },                             // F5
	func(cpu *Z80Cpu) { panic("CB Opcode F6 unimplemented") },                             // F6
	func(cpu *Z80Cpu) { panic("CB Opcode F7 unimplemented") },                             // F7
	func(cpu *Z80Cpu) { panic("CB Opcode F8 unimplemented") },                             // F8
	func(cpu *Z80Cpu) { panic("CB Opcode F9 unimplemented") },                             // F9
	func(cpu *Z80Cpu) { panic("CB Opcode FA unimplemented") },                             // FA
	func(cpu *Z80Cpu) { panic("CB Opcode FB unimplemented") },                             // FB
	func(cpu *Z80Cpu) { panic("CB Opcode FC unimplemented") },                             // FC
	func(cpu *Z80Cpu) { panic("CB Opcode FD unimplemented") },                             // FD
	func(cpu *Z80Cpu) { panic("CB Opcode FE unimplemented") },                             // FE
	func(cpu *Z80Cpu) { panic("CB Opcode FF unimplemented") },                             // FF
}

var cycles_opcode = []uint8{
	1, 3, 2, 2, 1, 1, 2, 1, 5, 2, 2, 2, 1, 1, 2, 1,
	1, 3, 2, 2, 1, 1, 2, 1, 3, 2, 2, 2, 1, 1, 2, 1,
	2, 3, 2, 2, 1, 1, 2, 1, 2, 2, 2, 2, 1, 1, 2, 1,
	2, 3, 2, 2, 3, 3, 3, 1, 2, 2, 2, 2, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	2, 2, 2, 2, 2, 2, 1, 2, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	2, 3, 3, 4, 3, 4, 2, 4, 2, 4, 3, 0, 3, 6, 2, 4,
	2, 3, 3, 0, 3, 4, 2, 4, 2, 4, 3, 0, 3, 0, 2, 4,
	3, 3, 2, 0, 0, 4, 2, 4, 4, 1, 4, 0, 0, 0, 2, 4,
	3, 3, 2, 1, 0, 4, 2, 4, 3, 2, 4, 1, 0, 0, 2, 4,
}

var cycles_branched = []uint8{
	1, 3, 2, 2, 1, 1, 2, 1, 5, 2, 2, 2, 1, 1, 2, 1,
	1, 3, 2, 2, 1, 1, 2, 1, 3, 2, 2, 2, 1, 1, 2, 1,
	3, 3, 2, 2, 1, 1, 2, 1, 3, 2, 2, 2, 1, 1, 2, 1,
	3, 3, 2, 2, 3, 3, 3, 1, 3, 2, 2, 2, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	2, 2, 2, 2, 2, 2, 1, 2, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1,
	5, 3, 4, 4, 6, 4, 2, 4, 5, 4, 4, 0, 6, 6, 2, 4,
	5, 3, 4, 0, 6, 4, 2, 4, 5, 4, 4, 0, 6, 0, 2, 4,
	3, 3, 2, 0, 0, 4, 2, 4, 4, 1, 4, 0, 0, 0, 2, 4,
	3, 3, 2, 1, 0, 4, 2, 4, 3, 2, 4, 1, 0, 0, 2, 4,
}

var cycles_cb = []uint8{
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
	2, 2, 2, 2, 2, 2, 3, 2, 2, 2, 2, 2, 2, 2, 3, 2,
	2, 2, 2, 2, 2, 2, 3, 2, 2, 2, 2, 2, 2, 2, 3, 2,
	2, 2, 2, 2, 2, 2, 3, 2, 2, 2, 2, 2, 2, 2, 3, 2,
	2, 2, 2, 2, 2, 2, 3, 2, 2, 2, 2, 2, 2, 2, 3, 2,
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
	2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 2, 2, 2, 2, 4, 2,
}
