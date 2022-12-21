package z80cpu

import (
	"fmt"
)

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
	A, B, C, D, E, H, L uint8
	SP, PC              uint16

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
	Disas       Z80Disas
}

func MakeZ80Cpu(mem Memory) *Z80Cpu {
	cpu := &Z80Cpu{
		Mem: mem,
	}
	cpu.Reset()
	return cpu
}

func (cpu *Z80Cpu) Reset() {
	cpu.SP = 0xff
	cpu.PC = 0
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
	opcode := cpu.Mem.Read(cpu.PC)
	cpu.PC += 1
	return opcode
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
	cpu.StackPush16(cpu.PC)
	cpu.interruptsEnabled = false

	interruptWasHandled := false
	for _, interrupt := range cpu.Interrupts {
		if interruptValue&interrupt.Mask != 0 {
			cpu.IF &= ^interrupt.Mask
			cpu.PC = interrupt.Addr
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
		_, disas_str := cpu.Disas.DisassembleOneFromCPU(cpu)
		fmt.Println(disas_str)
	}

	isCBOpcode := false
	cb_opcode := uint8(0)
	opcode := cpu.fetchOpcode()
	if opcode == 0xcb {
		isCBOpcode = true
		cb_opcode = cpu.Mem.Read(cpu.PC)
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
	v := cpu.Mem.Read(cpu.PC)

	cpu.PC += 1
	return v
}

func (cpu *Z80Cpu) getPC16() uint16 {
	l := cpu.Mem.Read(cpu.PC)
	h := cpu.Mem.Read(cpu.PC + 1)

	cpu.PC += 2
	return (uint16(h) << 8) | uint16(l)
}

func (cpu *Z80Cpu) StackPush16(val uint16) {
	cpu.SP -= 1
	cpu.Mem.Write(cpu.SP, uint8(val>>8))

	cpu.SP -= 1
	cpu.Mem.Write(cpu.SP, uint8(val&0xff))
}

func (cpu *Z80Cpu) StackPop16() uint16 {
	low := cpu.Mem.Read(cpu.SP)
	cpu.SP += 1
	high := cpu.Mem.Read(cpu.SP)
	cpu.SP += 1

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
	cpu.H, cpu.L = unpack_regcouple(pack_regcouple(cpu.H, cpu.L) + 1)
}

func handler_ldi_MEM_R(cpu *Z80Cpu, addr uint16, val uint8) {
	cpu.Mem.Write(addr, val)
	cpu.H, cpu.L = unpack_regcouple(pack_regcouple(cpu.H, cpu.L) + 1)
}

// LDD
func handler_ldd_R_MEM(cpu *Z80Cpu, dst *uint8, addr uint16) {
	*dst = cpu.Mem.Read(addr)
	cpu.H, cpu.L = unpack_regcouple(pack_regcouple(cpu.H, cpu.L) - 1)
}

func handler_ldd_MEM_R(cpu *Z80Cpu, addr uint16, val uint8) {
	cpu.Mem.Write(addr, val)
	cpu.H, cpu.L = unpack_regcouple(pack_regcouple(cpu.H, cpu.L) - 1)
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

func handler_inc_MEM(cpu *Z80Cpu, addr uint16) {
	val := cpu.Mem.Read(addr) + 1
	cpu.Mem.Write(addr, val)

	cpu.flagWasZero = val == 0
	cpu.flagWasSub = false
	cpu.flagHalfCarry = val&0xf == 0xf
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

func handler_dec_MEM(cpu *Z80Cpu, addr uint16) {
	val := cpu.Mem.Read(addr) - 1
	cpu.Mem.Write(addr, val)

	cpu.flagWasZero = val == 0
	cpu.flagWasSub = true
	cpu.flagHalfCarry = val&0xf == 0xf
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

// OR
func handler_or_R_8(cpu *Z80Cpu, dst *uint8, value uint8) {
	*dst = *dst | value

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
	cpu.A = ^cpu.A

	cpu.flagWasSub = true
	cpu.flagHalfCarry = true
}

// DAA
func handler_daa(cpu *Z80Cpu) {
	val := cpu.A

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

	cpu.A = val
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
	cpu.flagHalfCarry = true
	cpu.flagWasSub = false
}

// RES
func handler_res_R(cpu *Z80Cpu, bit int, dst *uint8) {
	var mask uint8 = ^(1 << bit)
	*dst &= mask
}

func handler_res_MEM(cpu *Z80Cpu, bit int, addr uint16) {
	var mask uint8 = ^(1 << bit)

	val := cpu.Mem.Read(addr)
	cpu.Mem.Write(addr, val&mask)
}

// SWAP
func handler_swap_R(cpu *Z80Cpu, dst *uint8) {
	low := *dst & 0xF
	hig := *dst >> 4
	*dst = (low << 4) | hig

	cpu.flagWasZero = *dst == 0
	cpu.flagCarry = false
	cpu.flagHalfCarry = false
	cpu.flagWasSub = false
}

func handler_swap_MEM(cpu *Z80Cpu, addr uint16) {
	val := cpu.Mem.Read(addr)
	low := val & 0xF
	hig := val >> 4
	val = (low << 4) | hig
	cpu.Mem.Write(addr, val)

	cpu.flagWasZero = val == 0
	cpu.flagCarry = false
	cpu.flagHalfCarry = false
	cpu.flagWasSub = false
}

// RET
func handler_ret(cpu *Z80Cpu) {
	cpu.PC = cpu.StackPop16()
}

func handler_ret_IF(cpu *Z80Cpu, cond bool) {
	if cond {
		cpu.branchWasTaken = true
		cpu.PC = cpu.StackPop16()
	}
}

// CALL
func handler_call(cpu *Z80Cpu) {
	addr := cpu.getPC16()
	cpu.StackPush16(cpu.PC)
	cpu.PC = addr
}

// JR
func handler_jr(cpu *Z80Cpu, off int8) {
	cpu.PC = uint16(int(cpu.PC) + 1 + int(off))
}

func handler_jr_IF(cpu *Z80Cpu, off int8, cond bool) {
	if cond {
		cpu.branchWasTaken = true
		cpu.PC = uint16(int(cpu.PC) + 1 + int(off))
	} else {
		cpu.PC += 1
	}
}

// JP
func handler_jp(cpu *Z80Cpu, dst uint16) {
	cpu.PC = dst
}

func handler_jp_IF(cpu *Z80Cpu, dst uint16, cond bool) {
	if cond {
		cpu.branchWasTaken = true
		cpu.PC = dst
	}
}

// OTHER
func handler_di(cpu *Z80Cpu) {
	cpu.interruptsEnabled = false
}

func handler_ei(cpu *Z80Cpu) {
	cpu.interruptsEnabled = true
}

func handler_rst(cpu *Z80Cpu, val uint16) {
	cpu.StackPush16(cpu.PC)
	cpu.PC = val
}

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
	func(cpu *Z80Cpu) { handler_ld_R_16(cpu, &cpu.B, &cpu.C, cpu.getPC16()) },                       // 01
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.B, cpu.C), cpu.A) },                // 02
	func(cpu *Z80Cpu) { handler_inc_R_16(cpu, &cpu.B, &cpu.C) },                                     // 03
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.B) },                                              // 04
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.B) },                                              // 05
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.B, cpu.getPC8()) },                                 // 06
	func(cpu *Z80Cpu) { handler_rlc_R(cpu, &cpu.A); cpu.flagWasZero = false /* rlca */ },            // 07
	func(cpu *Z80Cpu) { handler_ld_MEM_16(cpu, cpu.getPC16(), cpu.SP) },                             // 08
	func(cpu *Z80Cpu) { handler_add_R_16(cpu, &cpu.H, &cpu.L, pack_regcouple(cpu.B, cpu.C)) },       // 09
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.A, pack_regcouple(cpu.B, cpu.C)) },             // 0A
	func(cpu *Z80Cpu) { handler_dec_R_16(cpu, &cpu.B, &cpu.C) },                                     // 0B
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.C) },                                              // 0C
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.C) },                                              // 0D
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.C, cpu.getPC8()) },                                 // 0E
	func(cpu *Z80Cpu) { handler_rrc_R(cpu, &cpu.A); cpu.flagWasZero = false /* rrca */ },            // 0F
	func(cpu *Z80Cpu) { handler_stop() },                                                            // 10
	func(cpu *Z80Cpu) { handler_ld_R_16(cpu, &cpu.D, &cpu.E, cpu.getPC16()) },                       // 11
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.D, cpu.E), cpu.A) },                // 12
	func(cpu *Z80Cpu) { handler_inc_R_16(cpu, &cpu.D, &cpu.E) },                                     // 13
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.D) },                                              // 14
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.D) },                                              // 15
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.D, cpu.getPC8()) },                                 // 16
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.A); cpu.flagWasZero = false /* rla */ },              // 17
	func(cpu *Z80Cpu) { handler_jr(cpu, int8(cpu.Mem.Read(cpu.PC))) },                               // 18
	func(cpu *Z80Cpu) { handler_add_R_16(cpu, &cpu.H, &cpu.L, pack_regcouple(cpu.D, cpu.E)) },       // 19
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.A, pack_regcouple(cpu.D, cpu.E)) },             // 1A
	func(cpu *Z80Cpu) { handler_dec_R_16(cpu, &cpu.D, &cpu.E) },                                     // 1B
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.E) },                                              // 1C
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.E) },                                              // 1D
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.E, cpu.getPC8()) },                                 // 1E
	func(cpu *Z80Cpu) { handler_rr_R(cpu, &cpu.A); cpu.flagWasZero = false /* rra */ },              // 1F
	func(cpu *Z80Cpu) { handler_jr_IF(cpu, int8(cpu.Mem.Read(cpu.PC)), !cpu.flagWasZero) },          // 20
	func(cpu *Z80Cpu) { handler_ld_R_16(cpu, &cpu.H, &cpu.L, cpu.getPC16()) },                       // 21
	func(cpu *Z80Cpu) { handler_ldi_MEM_R(cpu, pack_regcouple(cpu.H, cpu.L), cpu.A) },               // 22
	func(cpu *Z80Cpu) { handler_inc_R_16(cpu, &cpu.H, &cpu.L) },                                     // 23
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.H) },                                              // 24
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.H) },                                              // 25
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.H, cpu.getPC8()) },                                 // 26
	func(cpu *Z80Cpu) { handler_daa(cpu) },                                                          // 27
	func(cpu *Z80Cpu) { handler_jr_IF(cpu, int8(cpu.Mem.Read(cpu.PC)), cpu.flagWasZero) },           // 28
	func(cpu *Z80Cpu) { handler_add_R_16(cpu, &cpu.H, &cpu.L, pack_regcouple(cpu.H, cpu.L)) },       // 29
	func(cpu *Z80Cpu) { handler_ldi_R_MEM(cpu, &cpu.A, pack_regcouple(cpu.H, cpu.L)) },              // 2A
	func(cpu *Z80Cpu) { handler_dec_R_16(cpu, &cpu.H, &cpu.L) },                                     // 2B
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.L) },                                              // 2C
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.L) },                                              // 2D
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.L, cpu.getPC8()) },                                 // 2E
	func(cpu *Z80Cpu) { handler_cpl(cpu) },                                                          // 2F
	func(cpu *Z80Cpu) { handler_jr_IF(cpu, int8(cpu.Mem.Read(cpu.PC)), !cpu.flagCarry) },            // 30
	func(cpu *Z80Cpu) { handler_ld_R_16_2(cpu, &cpu.SP, cpu.getPC16()) },                            // 31
	func(cpu *Z80Cpu) { handler_ldd_MEM_R(cpu, pack_regcouple(cpu.H, cpu.L), cpu.A) },               // 32
	func(cpu *Z80Cpu) { handler_inc_R_16_2(cpu, &cpu.SP) },                                          // 33
	func(cpu *Z80Cpu) { handler_inc_MEM(cpu, pack_regcouple(cpu.H, cpu.L)) },                        // 34
	func(cpu *Z80Cpu) { handler_dec_MEM(cpu, pack_regcouple(cpu.H, cpu.L)) },                        // 35
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.H, cpu.L), cpu.getPC8()) },         // 36
	func(cpu *Z80Cpu) { panic("Opcode 37 unimplemented") },                                          // 37
	func(cpu *Z80Cpu) { handler_jr_IF(cpu, int8(cpu.Mem.Read(cpu.PC)), cpu.flagCarry) },             // 38
	func(cpu *Z80Cpu) { handler_add_R_16(cpu, &cpu.H, &cpu.L, cpu.SP) },                             // 39
	func(cpu *Z80Cpu) { handler_ldd_R_MEM(cpu, &cpu.A, pack_regcouple(cpu.H, cpu.L)) },              // 3A
	func(cpu *Z80Cpu) { handler_dec_R_16_2(cpu, &cpu.SP) },                                          // 3B
	func(cpu *Z80Cpu) { handler_inc_R_8(cpu, &cpu.A) },                                              // 3C
	func(cpu *Z80Cpu) { handler_dec_R_8(cpu, &cpu.A) },                                              // 3D
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.A, cpu.getPC8()) },                                 // 3E
	func(cpu *Z80Cpu) { panic("Opcode 3F unimplemented") },                                          // 3F
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.B, cpu.B) },                                        // 40
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.B, cpu.C) },                                        // 41
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.B, cpu.D) },                                        // 42
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.B, cpu.E) },                                        // 43
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.B, cpu.H) },                                        // 44
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.B, cpu.L) },                                        // 45
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.B, pack_regcouple(cpu.H, cpu.L)) },             // 46
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.B, cpu.A) },                                        // 47
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.C, cpu.B) },                                        // 48
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.C, cpu.C) },                                        // 49
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.C, cpu.D) },                                        // 4A
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.C, cpu.E) },                                        // 4B
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.C, cpu.H) },                                        // 4C
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.C, cpu.L) },                                        // 4D
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.C, pack_regcouple(cpu.H, cpu.L)) },             // 4E
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.C, cpu.A) },                                        // 4F
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.D, cpu.B) },                                        // 50
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.D, cpu.C) },                                        // 51
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.D, cpu.D) },                                        // 52
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.D, cpu.E) },                                        // 53
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.D, cpu.H) },                                        // 54
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.D, cpu.L) },                                        // 55
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.D, pack_regcouple(cpu.H, cpu.L)) },             // 56
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.D, cpu.A) },                                        // 57
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.E, cpu.B) },                                        // 58
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.E, cpu.C) },                                        // 59
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.E, cpu.D) },                                        // 5A
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.E, cpu.E) },                                        // 5B
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.E, cpu.H) },                                        // 5C
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.E, cpu.L) },                                        // 5D
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.E, pack_regcouple(cpu.H, cpu.L)) },             // 5E
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.E, cpu.A) },                                        // 5F
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.H, cpu.B) },                                        // 60
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.H, cpu.C) },                                        // 61
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.H, cpu.D) },                                        // 62
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.H, cpu.E) },                                        // 63
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.H, cpu.H) },                                        // 64
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.H, cpu.L) },                                        // 65
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.H, pack_regcouple(cpu.H, cpu.L)) },             // 66
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.H, cpu.A) },                                        // 67
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.L, cpu.B) },                                        // 68
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.L, cpu.C) },                                        // 69
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.L, cpu.D) },                                        // 6A
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.L, cpu.E) },                                        // 6B
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.L, cpu.H) },                                        // 6C
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.L, cpu.L) },                                        // 6D
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.L, pack_regcouple(cpu.H, cpu.L)) },             // 6E
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.L, cpu.A) },                                        // 6F
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.H, cpu.L), cpu.B) },                // 70
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.H, cpu.L), cpu.C) },                // 71
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.H, cpu.L), cpu.D) },                // 72
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.H, cpu.L), cpu.E) },                // 73
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.H, cpu.L), cpu.H) },                // 74
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.H, cpu.L), cpu.L) },                // 75
	func(cpu *Z80Cpu) { handler_halt(cpu) },                                                         // 76
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, pack_regcouple(cpu.H, cpu.L), cpu.A) },                // 77
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.A, cpu.B) },                                        // 78
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.A, cpu.C) },                                        // 79
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.A, cpu.D) },                                        // 7A
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.A, cpu.E) },                                        // 7B
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.A, cpu.H) },                                        // 7C
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.A, cpu.L) },                                        // 7D
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.A, pack_regcouple(cpu.H, cpu.L)) },             // 7E
	func(cpu *Z80Cpu) { handler_ld_R_8(cpu, &cpu.A, cpu.A) },                                        // 7F
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.A, cpu.B) },                                       // 80
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.A, cpu.C) },                                       // 81
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.A, cpu.D) },                                       // 82
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.A, cpu.E) },                                       // 83
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.A, cpu.H) },                                       // 84
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.A, cpu.L) },                                       // 85
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.A, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) },  // 86
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.A, cpu.A) },                                       // 87
	func(cpu *Z80Cpu) { panic("Opcode 88 unimplemented") },                                          // 88
	func(cpu *Z80Cpu) { panic("Opcode 89 unimplemented") },                                          // 89
	func(cpu *Z80Cpu) { panic("Opcode 8A unimplemented") },                                          // 8A
	func(cpu *Z80Cpu) { panic("Opcode 8B unimplemented") },                                          // 8B
	func(cpu *Z80Cpu) { panic("Opcode 8C unimplemented") },                                          // 8C
	func(cpu *Z80Cpu) { panic("Opcode 8D unimplemented") },                                          // 8D
	func(cpu *Z80Cpu) { panic("Opcode 8E unimplemented") },                                          // 8E
	func(cpu *Z80Cpu) { panic("Opcode 8F unimplemented") },                                          // 8F
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.A, cpu.B) },                                       // 90
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.A, cpu.C) },                                       // 91
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.A, cpu.D) },                                       // 92
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.A, cpu.E) },                                       // 93
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.A, cpu.H) },                                       // 94
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.A, cpu.L) },                                       // 95
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.A, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) },  // 96
	func(cpu *Z80Cpu) { handler_sub_R_8(cpu, &cpu.A, cpu.A) },                                       // 97
	func(cpu *Z80Cpu) { panic("Opcode 98 unimplemented") },                                          // 98
	func(cpu *Z80Cpu) { panic("Opcode 99 unimplemented") },                                          // 99
	func(cpu *Z80Cpu) { panic("Opcode 9A unimplemented") },                                          // 9A
	func(cpu *Z80Cpu) { panic("Opcode 9B unimplemented") },                                          // 9B
	func(cpu *Z80Cpu) { panic("Opcode 9C unimplemented") },                                          // 9C
	func(cpu *Z80Cpu) { panic("Opcode 9D unimplemented") },                                          // 9D
	func(cpu *Z80Cpu) { panic("Opcode 9E unimplemented") },                                          // 9E
	func(cpu *Z80Cpu) { panic("Opcode 9F unimplemented") },                                          // 9F
	func(cpu *Z80Cpu) { handler_and_R_8(cpu, &cpu.A, cpu.B) },                                       // A0
	func(cpu *Z80Cpu) { handler_and_R_8(cpu, &cpu.A, cpu.C) },                                       // A1
	func(cpu *Z80Cpu) { handler_and_R_8(cpu, &cpu.A, cpu.D) },                                       // A2
	func(cpu *Z80Cpu) { handler_and_R_8(cpu, &cpu.A, cpu.E) },                                       // A3
	func(cpu *Z80Cpu) { handler_and_R_8(cpu, &cpu.A, cpu.H) },                                       // A4
	func(cpu *Z80Cpu) { handler_and_R_8(cpu, &cpu.A, cpu.L) },                                       // A5
	func(cpu *Z80Cpu) { handler_and_R_8(cpu, &cpu.A, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) },  // A6
	func(cpu *Z80Cpu) { handler_and_R_8(cpu, &cpu.A, cpu.A) },                                       // A7
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.A, cpu.B) },                                       // A8
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.A, cpu.C) },                                       // A9
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.A, cpu.D) },                                       // AA
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.A, cpu.E) },                                       // AB
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.A, cpu.H) },                                       // AC
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.A, cpu.L) },                                       // AD
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.A, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) },  // AE
	func(cpu *Z80Cpu) { handler_xor_R_8(cpu, &cpu.A, cpu.A) },                                       // AF
	func(cpu *Z80Cpu) { handler_or_R_8(cpu, &cpu.A, cpu.B) },                                        // B0
	func(cpu *Z80Cpu) { handler_or_R_8(cpu, &cpu.A, cpu.C) },                                        // B1
	func(cpu *Z80Cpu) { handler_or_R_8(cpu, &cpu.A, cpu.D) },                                        // B2
	func(cpu *Z80Cpu) { handler_or_R_8(cpu, &cpu.A, cpu.E) },                                        // B3
	func(cpu *Z80Cpu) { handler_or_R_8(cpu, &cpu.A, cpu.H) },                                        // B4
	func(cpu *Z80Cpu) { handler_or_R_8(cpu, &cpu.A, cpu.L) },                                        // B5
	func(cpu *Z80Cpu) { handler_or_R_8(cpu, &cpu.A, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) },   // B6
	func(cpu *Z80Cpu) { handler_or_R_8(cpu, &cpu.A, cpu.A) },                                        // B7
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.A, cpu.B) },                                             // B8
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.A, cpu.C) },                                             // B9
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.A, cpu.D) },                                             // BA
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.A, cpu.E) },                                             // BB
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.A, cpu.H) },                                             // BC
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.A, cpu.L) },                                             // BD
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.A, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) },        // BE
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.A, cpu.A) },                                             // BF
	func(cpu *Z80Cpu) { handler_ret_IF(cpu, !cpu.flagWasZero) },                                     // C0
	func(cpu *Z80Cpu) { cpu.B, cpu.C = unpack_regcouple(cpu.StackPop16()) },                         // C1
	func(cpu *Z80Cpu) { handler_jp_IF(cpu, cpu.getPC16(), !cpu.flagWasZero) },                       // C2
	func(cpu *Z80Cpu) { handler_jp(cpu, cpu.getPC16()) },                                            // C3
	func(cpu *Z80Cpu) { panic("Opcode C4 unimplemented") },                                          // C4
	func(cpu *Z80Cpu) { cpu.StackPush16(pack_regcouple(cpu.B, cpu.C)) },                             // C5
	func(cpu *Z80Cpu) { handler_add_R_8(cpu, &cpu.A, cpu.getPC8()) },                                // C6
	func(cpu *Z80Cpu) { handler_rst(cpu, 0) },                                                       // C7
	func(cpu *Z80Cpu) { handler_ret_IF(cpu, cpu.flagWasZero) },                                      // C8
	func(cpu *Z80Cpu) { handler_ret(cpu) },                                                          // C9
	func(cpu *Z80Cpu) { handler_jp_IF(cpu, cpu.getPC16(), cpu.flagWasZero) },                        // CA
	func(cpu *Z80Cpu) { cb_handlers[cpu.getPC8()](cpu) },                                            // CB
	func(cpu *Z80Cpu) { panic("Opcode CC unimplemented") },                                          // CC
	func(cpu *Z80Cpu) { handler_call(cpu) },                                                         // CD
	func(cpu *Z80Cpu) { handler_adc_R_8(cpu, &cpu.A, cpu.getPC8()) },                                // CE
	func(cpu *Z80Cpu) { handler_rst(cpu, 8) },                                                       // CF
	func(cpu *Z80Cpu) { handler_ret_IF(cpu, !cpu.flagCarry) },                                       // D0
	func(cpu *Z80Cpu) { cpu.D, cpu.E = unpack_regcouple(cpu.StackPop16()) },                         // D1
	func(cpu *Z80Cpu) { handler_jp_IF(cpu, cpu.getPC16(), !cpu.flagCarry) },                         // D2
	func(cpu *Z80Cpu) { handler_out(cpu, cpu.A) },                                                   // D3
	func(cpu *Z80Cpu) { panic("Opcode D4 unimplemented") },                                          // D4
	func(cpu *Z80Cpu) { cpu.StackPush16(pack_regcouple(cpu.D, cpu.E)) },                             // D5
	func(cpu *Z80Cpu) { panic("Opcode D6 unimplemented") },                                          // D6
	func(cpu *Z80Cpu) { handler_rst(cpu, 16) },                                                      // D7
	func(cpu *Z80Cpu) { handler_ret_IF(cpu, cpu.flagCarry) },                                        // D8
	func(cpu *Z80Cpu) { handler_ret(cpu); handler_ei(cpu) },                                         // D9
	func(cpu *Z80Cpu) { handler_jp_IF(cpu, cpu.getPC16(), !cpu.flagCarry) },                         // DA
	func(cpu *Z80Cpu) { panic("Opcode DB unimplemented") },                                          // DB
	func(cpu *Z80Cpu) { panic("Opcode DC unimplemented") },                                          // DC
	func(cpu *Z80Cpu) { panic("Opcode DD unimplemented") },                                          // DD
	func(cpu *Z80Cpu) { panic("Opcode DE unimplemented") },                                          // DE
	func(cpu *Z80Cpu) { handler_rst(cpu, 24) },                                                      // DF
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, uint16(cpu.getPC8())+0xFF00, cpu.A) },                 // E0
	func(cpu *Z80Cpu) { cpu.H, cpu.L = unpack_regcouple(cpu.StackPop16()) },                         // E1
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, uint16(cpu.C)+0xFF00, cpu.A) },                        // E2
	func(cpu *Z80Cpu) { panic("Opcode E3 unimplemented") },                                          // E3
	func(cpu *Z80Cpu) { panic("Opcode E4 unimplemented") },                                          // E4
	func(cpu *Z80Cpu) { cpu.StackPush16(pack_regcouple(cpu.H, cpu.L)) },                             // E5
	func(cpu *Z80Cpu) { handler_and_R_8(cpu, &cpu.A, cpu.getPC8()) },                                // E6
	func(cpu *Z80Cpu) { handler_rst(cpu, 32) },                                                      // E7
	func(cpu *Z80Cpu) { panic("Opcode E8 unimplemented") },                                          // E8
	func(cpu *Z80Cpu) { handler_jp(cpu, pack_regcouple(cpu.H, cpu.L)) },                             // E9
	func(cpu *Z80Cpu) { handler_ld_MEM_8(cpu, cpu.getPC16(), cpu.A) },                               // EA
	func(cpu *Z80Cpu) { panic("Opcode EB unimplemented") },                                          // EB
	func(cpu *Z80Cpu) { panic("Opcode EC unimplemented") },                                          // EC
	func(cpu *Z80Cpu) { panic("Opcode ED unimplemented") },                                          // ED
	func(cpu *Z80Cpu) { panic("Opcode EE unimplemented") },                                          // EE
	func(cpu *Z80Cpu) { handler_rst(cpu, 40) },                                                      // EF
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.A, uint16(cpu.getPC8())+0xFF00) },              // F0
	func(cpu *Z80Cpu) { a, f := unpack_regcouple(cpu.StackPop16()); cpu.A = a; cpu.UnpackFlags(f) }, // F1
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.A, uint16(cpu.C)+0xFF00) },                     // F2
	func(cpu *Z80Cpu) { handler_di(cpu) },                                                           // F3
	func(cpu *Z80Cpu) { panic("Opcode F4 unimplemented") },                                          // F4
	func(cpu *Z80Cpu) { cpu.StackPush16(pack_regcouple(cpu.A, cpu.PackFlags())) },                   // F5
	func(cpu *Z80Cpu) { panic("Opcode F6 unimplemented") },                                          // F6
	func(cpu *Z80Cpu) { handler_rst(cpu, 48) },                                                      // F7
	func(cpu *Z80Cpu) { panic("Opcode F8 unimplemented") },                                          // F8
	func(cpu *Z80Cpu) { panic("Opcode F9 unimplemented") },                                          // F9
	func(cpu *Z80Cpu) { handler_ld_R_MEM_8(cpu, &cpu.A, cpu.getPC16()) },                            // FA
	func(cpu *Z80Cpu) { handler_ei(cpu) },                                                           // FB
	func(cpu *Z80Cpu) { panic("Opcode FC unimplemented") },                                          // FC
	func(cpu *Z80Cpu) { panic("Opcode FD unimplemented") },                                          // FD
	func(cpu *Z80Cpu) { handler_cp(cpu, cpu.A, cpu.getPC8()) },                                      // FE
	func(cpu *Z80Cpu) { handler_rst(cpu, 56) },                                                      // FF
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
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.B) },                                       // 10
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.C) },                                       // 11
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.D) },                                       // 12
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.E) },                                       // 13
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.H) },                                       // 14
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.L) },                                       // 15
	func(cpu *Z80Cpu) { handler_rl_MEM(cpu, pack_regcouple(cpu.H, cpu.L)) },               // 16
	func(cpu *Z80Cpu) { handler_rl_R(cpu, &cpu.A) },                                       // 17
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
	func(cpu *Z80Cpu) { handler_swap_R(cpu, &cpu.B) },                                     // 30
	func(cpu *Z80Cpu) { handler_swap_R(cpu, &cpu.C) },                                     // 31
	func(cpu *Z80Cpu) { handler_swap_R(cpu, &cpu.D) },                                     // 32
	func(cpu *Z80Cpu) { handler_swap_R(cpu, &cpu.E) },                                     // 33
	func(cpu *Z80Cpu) { handler_swap_R(cpu, &cpu.H) },                                     // 34
	func(cpu *Z80Cpu) { handler_swap_R(cpu, &cpu.L) },                                     // 35
	func(cpu *Z80Cpu) { handler_swap_MEM(cpu, pack_regcouple(cpu.H, cpu.L)) },             // 36
	func(cpu *Z80Cpu) { handler_swap_R(cpu, &cpu.A) },                                     // 37
	func(cpu *Z80Cpu) { panic("CB Opcode 38 unimplemented") },                             // 38
	func(cpu *Z80Cpu) { panic("CB Opcode 39 unimplemented") },                             // 39
	func(cpu *Z80Cpu) { panic("CB Opcode 3A unimplemented") },                             // 3A
	func(cpu *Z80Cpu) { panic("CB Opcode 3B unimplemented") },                             // 3B
	func(cpu *Z80Cpu) { panic("CB Opcode 3C unimplemented") },                             // 3C
	func(cpu *Z80Cpu) { panic("CB Opcode 3D unimplemented") },                             // 3D
	func(cpu *Z80Cpu) { panic("CB Opcode 3E unimplemented") },                             // 3E
	func(cpu *Z80Cpu) { panic("CB Opcode 3F unimplemented") },                             // 3F
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.B) },                                      // 40
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.C) },                                      // 41
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.D) },                                      // 42
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.E) },                                      // 43
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.H) },                                      // 44
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.L) },                                      // 45
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) }, // 46
	func(cpu *Z80Cpu) { handler_bit(cpu, 0, cpu.A) },                                      // 47
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.B) },                                      // 48
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.C) },                                      // 49
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.D) },                                      // 4A
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.E) },                                      // 4B
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.H) },                                      // 4C
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.L) },                                      // 4D
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) }, // 4E
	func(cpu *Z80Cpu) { handler_bit(cpu, 1, cpu.A) },                                      // 4F
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.B) },                                      // 50
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.C) },                                      // 51
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.D) },                                      // 52
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.E) },                                      // 53
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.H) },                                      // 54
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.L) },                                      // 55
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) }, // 56
	func(cpu *Z80Cpu) { handler_bit(cpu, 2, cpu.A) },                                      // 57
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.B) },                                      // 58
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.C) },                                      // 59
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.D) },                                      // 5A
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.E) },                                      // 5B
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.H) },                                      // 5C
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.L) },                                      // 5D
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) }, // 5E
	func(cpu *Z80Cpu) { handler_bit(cpu, 3, cpu.A) },                                      // 5F
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.B) },                                      // 60
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.C) },                                      // 61
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.D) },                                      // 62
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.E) },                                      // 63
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.H) },                                      // 64
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.L) },                                      // 65
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) }, // 66
	func(cpu *Z80Cpu) { handler_bit(cpu, 4, cpu.A) },                                      // 67
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.B) },                                      // 68
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.C) },                                      // 69
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.D) },                                      // 6A
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.E) },                                      // 6B
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.H) },                                      // 6C
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.L) },                                      // 6D
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) }, // 6E
	func(cpu *Z80Cpu) { handler_bit(cpu, 5, cpu.A) },                                      // 6F
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.B) },                                      // 70
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.C) },                                      // 71
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.D) },                                      // 72
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.E) },                                      // 73
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.H) },                                      // 74
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.L) },                                      // 75
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) }, // 76
	func(cpu *Z80Cpu) { handler_bit(cpu, 6, cpu.A) },                                      // 77
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.B) },                                      // 78
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.C) },                                      // 79
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.D) },                                      // 7A
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.E) },                                      // 7B
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.H) },                                      // 7C
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.L) },                                      // 7D
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.Mem.Read(pack_regcouple(cpu.H, cpu.L))) }, // 7E
	func(cpu *Z80Cpu) { handler_bit(cpu, 7, cpu.A) },                                      // 7F
	func(cpu *Z80Cpu) { handler_res_R(cpu, 0, &cpu.B) },                                   // 80
	func(cpu *Z80Cpu) { handler_res_R(cpu, 0, &cpu.C) },                                   // 81
	func(cpu *Z80Cpu) { handler_res_R(cpu, 0, &cpu.D) },                                   // 82
	func(cpu *Z80Cpu) { handler_res_R(cpu, 0, &cpu.E) },                                   // 83
	func(cpu *Z80Cpu) { handler_res_R(cpu, 0, &cpu.H) },                                   // 84
	func(cpu *Z80Cpu) { handler_res_R(cpu, 0, &cpu.L) },                                   // 85
	func(cpu *Z80Cpu) { handler_res_MEM(cpu, 0, pack_regcouple(cpu.H, cpu.L)) },           // 86
	func(cpu *Z80Cpu) { handler_res_R(cpu, 0, &cpu.A) },                                   // 87
	func(cpu *Z80Cpu) { handler_res_R(cpu, 1, &cpu.B) },                                   // 88
	func(cpu *Z80Cpu) { handler_res_R(cpu, 1, &cpu.C) },                                   // 89
	func(cpu *Z80Cpu) { handler_res_R(cpu, 1, &cpu.D) },                                   // 8A
	func(cpu *Z80Cpu) { handler_res_R(cpu, 1, &cpu.E) },                                   // 8B
	func(cpu *Z80Cpu) { handler_res_R(cpu, 1, &cpu.H) },                                   // 8C
	func(cpu *Z80Cpu) { handler_res_R(cpu, 1, &cpu.L) },                                   // 8D
	func(cpu *Z80Cpu) { handler_res_MEM(cpu, 1, pack_regcouple(cpu.H, cpu.L)) },           // 8E
	func(cpu *Z80Cpu) { handler_res_R(cpu, 1, &cpu.A) },                                   // 8F
	func(cpu *Z80Cpu) { handler_res_R(cpu, 2, &cpu.B) },                                   // 90
	func(cpu *Z80Cpu) { handler_res_R(cpu, 2, &cpu.C) },                                   // 91
	func(cpu *Z80Cpu) { handler_res_R(cpu, 2, &cpu.D) },                                   // 92
	func(cpu *Z80Cpu) { handler_res_R(cpu, 2, &cpu.E) },                                   // 93
	func(cpu *Z80Cpu) { handler_res_R(cpu, 2, &cpu.H) },                                   // 94
	func(cpu *Z80Cpu) { handler_res_R(cpu, 2, &cpu.L) },                                   // 95
	func(cpu *Z80Cpu) { handler_res_MEM(cpu, 2, pack_regcouple(cpu.H, cpu.L)) },           // 96
	func(cpu *Z80Cpu) { handler_res_R(cpu, 2, &cpu.A) },                                   // 97
	func(cpu *Z80Cpu) { handler_res_R(cpu, 3, &cpu.B) },                                   // 98
	func(cpu *Z80Cpu) { handler_res_R(cpu, 3, &cpu.C) },                                   // 99
	func(cpu *Z80Cpu) { handler_res_R(cpu, 3, &cpu.D) },                                   // 9A
	func(cpu *Z80Cpu) { handler_res_R(cpu, 3, &cpu.E) },                                   // 9B
	func(cpu *Z80Cpu) { handler_res_R(cpu, 3, &cpu.H) },                                   // 9C
	func(cpu *Z80Cpu) { handler_res_R(cpu, 3, &cpu.L) },                                   // 9D
	func(cpu *Z80Cpu) { handler_res_MEM(cpu, 3, pack_regcouple(cpu.H, cpu.L)) },           // 9E
	func(cpu *Z80Cpu) { handler_res_R(cpu, 3, &cpu.A) },                                   // 9F
	func(cpu *Z80Cpu) { handler_res_R(cpu, 4, &cpu.B) },                                   // A0
	func(cpu *Z80Cpu) { handler_res_R(cpu, 4, &cpu.C) },                                   // A1
	func(cpu *Z80Cpu) { handler_res_R(cpu, 4, &cpu.D) },                                   // A2
	func(cpu *Z80Cpu) { handler_res_R(cpu, 4, &cpu.E) },                                   // A3
	func(cpu *Z80Cpu) { handler_res_R(cpu, 4, &cpu.H) },                                   // A4
	func(cpu *Z80Cpu) { handler_res_R(cpu, 4, &cpu.L) },                                   // A5
	func(cpu *Z80Cpu) { handler_res_MEM(cpu, 4, pack_regcouple(cpu.H, cpu.L)) },           // A6
	func(cpu *Z80Cpu) { handler_res_R(cpu, 4, &cpu.A) },                                   // A7
	func(cpu *Z80Cpu) { handler_res_R(cpu, 5, &cpu.B) },                                   // A8
	func(cpu *Z80Cpu) { handler_res_R(cpu, 5, &cpu.C) },                                   // A9
	func(cpu *Z80Cpu) { handler_res_R(cpu, 5, &cpu.D) },                                   // AA
	func(cpu *Z80Cpu) { handler_res_R(cpu, 5, &cpu.E) },                                   // AB
	func(cpu *Z80Cpu) { handler_res_R(cpu, 5, &cpu.H) },                                   // AC
	func(cpu *Z80Cpu) { handler_res_R(cpu, 5, &cpu.L) },                                   // AD
	func(cpu *Z80Cpu) { handler_res_MEM(cpu, 5, pack_regcouple(cpu.H, cpu.L)) },           // AE
	func(cpu *Z80Cpu) { handler_res_R(cpu, 5, &cpu.A) },                                   // AF
	func(cpu *Z80Cpu) { handler_res_R(cpu, 6, &cpu.B) },                                   // B0
	func(cpu *Z80Cpu) { handler_res_R(cpu, 6, &cpu.C) },                                   // B1
	func(cpu *Z80Cpu) { handler_res_R(cpu, 6, &cpu.D) },                                   // B2
	func(cpu *Z80Cpu) { handler_res_R(cpu, 6, &cpu.E) },                                   // B3
	func(cpu *Z80Cpu) { handler_res_R(cpu, 6, &cpu.H) },                                   // B4
	func(cpu *Z80Cpu) { handler_res_R(cpu, 6, &cpu.L) },                                   // B5
	func(cpu *Z80Cpu) { handler_res_MEM(cpu, 6, pack_regcouple(cpu.H, cpu.L)) },           // B6
	func(cpu *Z80Cpu) { handler_res_R(cpu, 6, &cpu.A) },                                   // B7
	func(cpu *Z80Cpu) { handler_res_R(cpu, 7, &cpu.B) },                                   // B8
	func(cpu *Z80Cpu) { handler_res_R(cpu, 7, &cpu.C) },                                   // B9
	func(cpu *Z80Cpu) { handler_res_R(cpu, 7, &cpu.D) },                                   // BA
	func(cpu *Z80Cpu) { handler_res_R(cpu, 7, &cpu.E) },                                   // BB
	func(cpu *Z80Cpu) { handler_res_R(cpu, 7, &cpu.H) },                                   // BC
	func(cpu *Z80Cpu) { handler_res_R(cpu, 7, &cpu.L) },                                   // BD
	func(cpu *Z80Cpu) { handler_res_MEM(cpu, 7, pack_regcouple(cpu.H, cpu.L)) },           // BE
	func(cpu *Z80Cpu) { handler_res_R(cpu, 7, &cpu.A) },                                   // BF
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
