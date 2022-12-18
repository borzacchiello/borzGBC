package z80cpu

import (
	"fmt"
	"strings"
)

type Z80Disas struct {
	addr uint16
	off  uint16
	data []byte
	cpu  *Z80Cpu
}

type Z80DisasError string

func (err Z80DisasError) Error() string {
	return string(err)
}

func (disas *Z80Disas) getByte() (error, uint8) {
	if disas.data != nil && disas.off >= uint16(len(disas.data)) {
		return Z80DisasError("Unable to get byte from data"), 0
	}

	var v uint8
	if disas.data != nil {
		v = disas.data[disas.off]
	} else {
		v = disas.cpu.Mem.Read(disas.off)
	}

	disas.off += 1
	return nil, v
}

func (disas *Z80Disas) getWord() (error, uint16) {
	if disas.data != nil && (disas.off >= uint16(len(disas.data)) || disas.off+1 >= uint16(len(disas.data))) {
		return Z80DisasError("Unable to get word from data"), 0
	}

	err, l := disas.getByte()
	if err != nil {
		return err, 0
	}
	err, h := disas.getByte()
	if err != nil {
		return err, 0
	}

	disas.off += 2
	return nil, (uint16(h) << 8) | uint16(l)
}

func (disas *Z80Disas) disassembleOne() (error, int, string) {
	var instr string

	err, opcode := disas.getByte()
	if err != nil {
		return err, 0, ""
	}

	if opcode == 0xcb {
		err, cb_opcode := disas.getByte()
		if err != nil {
			return err, 0, ""
		}

		instr = cbOpcodeTable[cb_opcode]
	} else {
		instr = opcodeTable[opcode]
	}

	if strings.Contains(instr, "nn") {
		err, val := disas.getWord()
		if err != nil {
			return err, 0, ""
		}

		val_str := fmt.Sprintf("$%04x", val)
		instr = strings.Replace(instr, "nn", val_str, 1)
	} else if strings.Contains(instr, "n") {
		err, val := disas.getByte()
		if err != nil {
			return err, 0, ""
		}

		val_str := fmt.Sprintf("$%02x", val)
		instr = strings.Replace(instr, "n", val_str, 1)
	}

	instr = fmt.Sprintf("%04x: %s", disas.addr, instr)
	return nil, int(disas.off), instr
}

func (disas *Z80Disas) DisassembleOneFromData(addr uint16, data []byte) (int, string) {
	disas.off = 0
	disas.cpu = nil
	disas.data = data
	disas.addr = addr

	err, off, str := disas.disassembleOne()
	if err != nil {
		return 0, "XXX Disasm Error"
	}
	return off, str
}

func (disas *Z80Disas) DisassembleOneFromCPU(cpu *Z80Cpu) (int, string) {
	disas.off = cpu.PC
	disas.cpu = cpu
	disas.data = nil
	disas.addr = cpu.PC

	err, off, str := disas.disassembleOne()
	if err != nil {
		return 0, "XXX Disasm Error"
	}
	return off, str
}

var opcodeTable = []string{
	"NOP",        // 00
	"LD BC,nn",   // 01
	"LD (BC),A",  // 02
	"INC BC",     // 03
	"INC B",      // 04
	"DEC B",      // 05
	"LD B,n",     // 06
	"RLC A",      // 07
	"LD (nn),SP", // 08
	"ADD HL,BC",  // 09
	"LD A,(BC)",  // 0a
	"DEC BC",     // 0b
	"INC C",      // 0c
	"DEC C",      // 0d
	"LD C,n",     // 0e
	"RRC A",      // 0f
	"STOP",       // 10
	"LD DE,nn",   // 11
	"LD (DE),A",  // 12
	"INC DE",     // 13
	"INC D",      // 14
	"DEC D",      // 15
	"LD D,n",     // 16
	"RL A",       // 17
	"JR n",       // 18
	"ADD HL,DE",  // 19
	"LD A,(DE)",  // 1a
	"DEC DE",     // 1b
	"INC E",      // 1c
	"DEC E",      // 1d
	"LD E,n",     // 1e
	"RR A",       // 1f
	"JR NZ,n",    // 20
	"LD HL,nn",   // 21
	"LDI (HL),A", // 22
	"INC HL",     // 23
	"INC H",      // 24
	"DEC H",      // 25
	"LD H,n",     // 26
	"DAA",        // 27
	"JR Z,n",     // 28
	"ADD HL,HL",  // 29
	"LDI A,(HL)", // 2a
	"DEC HL",     // 2b
	"INC L",      // 2c
	"DEC L",      // 2d
	"LD L,n",     // 2e
	"CPL",        // 2f
	"JR NC,n",    // 30
	"LD SP,nn",   // 31
	"LDD (HL),A", // 32
	"INC SP",     // 33
	"INC (HL)",   // 34
	"DEC (HL)",   // 35
	"LD (HL),n",  // 36
	"SCF",        // 37
	"JR C,n",     // 38
	"ADD HL,SP",  // 39
	"LDD A,(HL)", // 3a
	"DEC SP",     // 3b
	"INC A",      // 3c
	"DEC A",      // 3d
	"LD A,n",     // 3e
	"CCF",        // 3f
	"LD B,B",     // 40
	"LD B,C",     // 41
	"LD B,D",     // 42
	"LD B,E",     // 43
	"LD B,H",     // 44
	"LD B,L",     // 45
	"LD B,(HL)",  // 46
	"LD B,A",     // 47
	"LD C,B",     // 48
	"LD C,C",     // 49
	"LD C,D",     // 4a
	"LD C,E",     // 4b
	"LD C,H",     // 4c
	"LD C,L",     // 4d
	"LD C,(HL)",  // 4e
	"LD C,A",     // 4f
	"LD D,B",     // 50
	"LD D,C",     // 51
	"LD D,D",     // 52
	"LD D,E",     // 53
	"LD D,H",     // 54
	"LD D,L",     // 55
	"LD D,(HL)",  // 56
	"LD D,A",     // 57
	"LD E,B",     // 58
	"LD E,C",     // 59
	"LD E,D",     // 5a
	"LD E,E",     // 5b
	"LD E,H",     // 5c
	"LD E,L",     // 5d
	"LD E,(HL)",  // 5e
	"LD E,A",     // 5f
	"LD H,B",     // 60
	"LD H,C",     // 61
	"LD H,D",     // 62
	"LD H,E",     // 63
	"LD H,H",     // 64
	"LD H,L",     // 65
	"LD H,(HL)",  // 66
	"LD H,A",     // 67
	"LD L,B",     // 68
	"LD L,C",     // 69
	"LD L,D",     // 6a
	"LD L,E",     // 6b
	"LD L,H",     // 6c
	"LD L,L",     // 6d
	"LD L,(HL)",  // 6e
	"LD L,A",     // 6f
	"LD (HL),B",  // 70
	"LD (HL),C",  // 71
	"LD (HL),D",  // 72
	"LD (HL),E",  // 73
	"LD (HL),H",  // 74
	"LD (HL),L",  // 75
	"HALT",       // 76
	"LD (HL),A",  // 77
	"LD A,B",     // 78
	"LD A,C",     // 79
	"LD A,D",     // 7a
	"LD A,E",     // 7b
	"LD A,H",     // 7c
	"LD A,L",     // 7d
	"LD A,(HL)",  // 7e
	"LD A,A",     // 7f
	"ADD A,B",    // 80
	"ADD A,C",    // 81
	"ADD A,D",    // 82
	"ADD A,E",    // 83
	"ADD A,H",    // 84
	"ADD A,L",    // 85
	"ADD A,(HL)", // 86
	"ADD A,A",    // 87
	"ADC A,B",    // 88
	"ADC A,C",    // 89
	"ADC A,D",    // 8a
	"ADC A,E",    // 8b
	"ADC A,H",    // 8c
	"ADC A,L",    // 8d
	"ADC A,(HL)", // 8e
	"ADC A,A",    // 8f
	"SUB A,B",    // 90
	"SUB A,C",    // 91
	"SUB A,D",    // 92
	"SUB A,E",    // 93
	"SUB A,H",    // 94
	"SUB A,L",    // 95
	"SUB A,(HL)", // 96
	"SUB A,A",    // 97
	"SBC A,B",    // 98
	"SBC A,C",    // 99
	"SBC A,D",    // 9a
	"SBC A,E",    // 9b
	"SBC A,H",    // 9c
	"SBC A,L",    // 9d
	"SBC A,(HL)", // 9e
	"SBC A,A",    // 9f
	"AND B",      // a0
	"AND C",      // a1
	"AND D",      // a2
	"AND E",      // a3
	"AND H",      // a4
	"AND L",      // a5
	"AND (HL)",   // a6
	"AND A",      // a7
	"XOR B",      // a8
	"XOR C",      // a9
	"XOR D",      // aa
	"XOR E",      // ab
	"XOR H",      // ac
	"XOR L",      // ad
	"XOR (HL)",   // ae
	"XOR A",      // af
	"OR B",       // b0
	"OR C",       // b1
	"OR D",       // b2
	"OR E",       // b3
	"OR H",       // b4
	"OR L",       // b5
	"OR (HL)",    // b6
	"OR A",       // b7
	"CP B",       // b8
	"CP C",       // b9
	"CP D",       // ba
	"CP E",       // bb
	"CP H",       // bc
	"CP L",       // bd
	"CP (HL)",    // be
	"CP A",       // bf
	"RET NZ",     // c0
	"POP BC",     // c1
	"JP NZ,nn",   // c2
	"JP nn",      // c3
	"CALL NZ,nn", // c4
	"PUSH BC",    // c5
	"ADD A,n",    // c6
	"RST 0",      // c7
	"RET Z",      // c8
	"RET",        // c9
	"JP Z,nn",    // ca
	"Ext ops",    // cb
	"CALL Z,nn",  // cc
	"CALL nn",    // cd
	"ADC A,n",    // ce
	"RST 8",      // cf
	"RET NC",     // d0
	"POP DE",     // d1
	"JP NC,nn",   // d2
	"XX",         // d3
	"CALL NC,nn", // d4
	"PUSH DE",    // d5
	"SUB A,n",    // d6
	"RST 10",     // d7
	"RET C",      // d8
	"RETI",       // d9
	"JP C,nn",    // da
	"XX",         // db
	"CALL C,nn",  // dc
	"XX",         // dd
	"SBC A,n",    // de
	"RST 18",     // df
	"LDH (n),A",  // e0
	"POP HL",     // e1
	"LDH (C),A",  // e2
	"XX",         // e3
	"XX",         // e4
	"PUSH HL",    // e5
	"AND n",      // e6
	"RST 20",     // e7
	"ADD SP,d",   // e8
	"JP (HL)",    // e9
	"LD (nn),A",  // ea
	"XX",         // eb
	"XX",         // ec
	"XX",         // ed
	"XOR n",      // ee
	"RST 28",     // ef
	"LDH A,(n)",  // f0
	"POP AF",     // f1
	"XX",         // f2
	"DI",         // f3
	"XX",         // f4
	"PUSH AF",    // f5
	"OR n",       // f6
	"RST 30",     // f7
	"LDHL SP,d",  // f8
	"LD SP,HL",   // f9
	"LD A,(nn)",  // fa
	"EI",         // fb
	"XX",         // fc
	"XX",         // fd
	"CP n",       // fe
	"RST 38",     // ff
}

var cbOpcodeTable = []string{
	"RLC B",      // 00
	"RLC C",      // 01
	"RLC D",      // 02
	"RLC E",      // 03
	"RLC H",      // 04
	"RLC L",      // 05
	"RLC (HL)",   // 06
	"RLC A",      // 07
	"RRC B",      // 08
	"RRC C",      // 09
	"RRC D",      // 0a
	"RRC E",      // 0b
	"RRC H",      // 0c
	"RRC L",      // 0d
	"RRC (HL)",   // 0e
	"RRC A",      // 0f
	"RL B",       // 10
	"RL C",       // 11
	"RL D",       // 12
	"RL E",       // 13
	"RL H",       // 14
	"RL L",       // 15
	"RL (HL)",    // 16
	"RL A",       // 17
	"RR B",       // 18
	"RR C",       // 19
	"RR D",       // 1a
	"RR E",       // 1b
	"RR H",       // 1c
	"RR L",       // 1d
	"RR (HL)",    // 1e
	"RR A",       // 1f
	"SLA B",      // 20
	"SLA C",      // 21
	"SLA D",      // 22
	"SLA E",      // 23
	"SLA H",      // 24
	"SLA L",      // 25
	"SLA (HL)",   // 26
	"SLA A",      // 27
	"SRA B",      // 28
	"SRA C",      // 29
	"SRA D",      // 2a
	"SRA E",      // 2b
	"SRA H",      // 2c
	"SRA L",      // 2d
	"SRA (HL)",   // 2e
	"SRA A",      // 2f
	"SWAP B",     // 30
	"SWAP C",     // 31
	"SWAP D",     // 32
	"SWAP E",     // 33
	"SWAP H",     // 34
	"SWAP L",     // 35
	"SWAP (HL)",  // 36
	"SWAP A",     // 37
	"SRL B",      // 38
	"SRL C",      // 39
	"SRL D",      // 3a
	"SRL E",      // 3b
	"SRL H",      // 3c
	"SRL L",      // 3d
	"SRL (HL)",   // 3e
	"SRL A",      // 3f
	"BIT 0,B",    // 40
	"BIT 0,C",    // 41
	"BIT 0,D",    // 42
	"BIT 0,E",    // 43
	"BIT 0,H",    // 44
	"BIT 0,L",    // 45
	"BIT 0,(HL)", // 46
	"BIT 0,A",    // 47
	"BIT 1,B",    // 48
	"BIT 1,C",    // 49
	"BIT 1,D",    // 4a
	"BIT 1,E",    // 4b
	"BIT 1,H",    // 4c
	"BIT 1,L",    // 4d
	"BIT 1,(HL)", // 4e
	"BIT 1,A",    // 4f
	"BIT 2,B",    // 50
	"BIT 2,C",    // 51
	"BIT 2,D",    // 52
	"BIT 2,E",    // 53
	"BIT 2,H",    // 54
	"BIT 2,L",    // 55
	"BIT 2,(HL)", // 56
	"BIT 2,A",    // 57
	"BIT 3,B",    // 58
	"BIT 3,C",    // 59
	"BIT 3,D",    // 5a
	"BIT 3,E",    // 5b
	"BIT 3,H",    // 5c
	"BIT 3,L",    // 5d
	"BIT 3,(HL)", // 5e
	"BIT 3,A",    // 5f
	"BIT 4,B",    // 60
	"BIT 4,C",    // 61
	"BIT 4,D",    // 62
	"BIT 4,E",    // 63
	"BIT 4,H",    // 64
	"BIT 4,L",    // 65
	"BIT 4,(HL)", // 66
	"BIT 4,A",    // 67
	"BIT 5,B",    // 68
	"BIT 5,C",    // 69
	"BIT 5,D",    // 6a
	"BIT 5,E",    // 6b
	"BIT 5,H",    // 6c
	"BIT 5,L",    // 6d
	"BIT 5,(HL)", // 6e
	"BIT 5,A",    // 6f
	"BIT 6,B",    // 70
	"BIT 6,C",    // 71
	"BIT 6,D",    // 72
	"BIT 6,E",    // 73
	"BIT 6,H",    // 74
	"BIT 6,L",    // 75
	"BIT 6,(HL)", // 76
	"BIT 6,A",    // 77
	"BIT 7,B",    // 78
	"BIT 7,C",    // 79
	"BIT 7,D",    // 7a
	"BIT 7,E",    // 7b
	"BIT 7,H",    // 7c
	"BIT 7,L",    // 7d
	"BIT 7,(HL)", // 7e
	"BIT 7,A",    // 7f
	"RES 0,B",    // 80
	"RES 0,C",    // 81
	"RES 0,D",    // 82
	"RES 0,E",    // 83
	"RES 0,H",    // 84
	"RES 0,L",    // 85
	"RES 0,(HL)", // 86
	"RES 0,A",    // 87
	"RES 1,B",    // 88
	"RES 1,C",    // 89
	"RES 1,D",    // 8a
	"RES 1,E",    // 8b
	"RES 1,H",    // 8c
	"RES 1,L",    // 8d
	"RES 1,(HL)", // 8e
	"RES 1,A",    // 8f
	"RES 2,B",    // 90
	"RES 2,C",    // 91
	"RES 2,D",    // 92
	"RES 2,E",    // 93
	"RES 2,H",    // 94
	"RES 2,L",    // 95
	"RES 2,(HL)", // 96
	"RES 2,A",    // 97
	"RES 3,B",    // 98
	"RES 3,C",    // 99
	"RES 3,D",    // 9a
	"RES 3,E",    // 9b
	"RES 3,H",    // 9c
	"RES 3,L",    // 9d
	"RES 3,(HL)", // 9e
	"RES 3,A",    // 9f
	"RES 4,B",    // a0
	"RES 4,C",    // a1
	"RES 4,D",    // a2
	"RES 4,E",    // a3
	"RES 4,H",    // a4
	"RES 4,L",    // a5
	"RES 4,(HL)", // a6
	"RES 4,A",    // a7
	"RES 5,B",    // a8
	"RES 5,C",    // a9
	"RES 5,D",    // aa
	"RES 5,E",    // ab
	"RES 5,H",    // ac
	"RES 5,L",    // ad
	"RES 5,(HL)", // ae
	"RES 5,A",    // af
	"RES 6,B",    // b0
	"RES 6,C",    // b1
	"RES 6,D",    // b2
	"RES 6,E",    // b3
	"RES 6,H",    // b4
	"RES 6,L",    // b5
	"RES 6,(HL)", // b6
	"RES 6,A",    // b7
	"RES 7,B",    // b8
	"RES 7,C",    // b9
	"RES 7,D",    // ba
	"RES 7,E",    // bb
	"RES 7,H",    // bc
	"RES 7,L",    // bd
	"RES 7,(HL)", // be
	"RES 7,A",    // bf
	"SET 0,B",    // c0
	"SET 0,C",    // c1
	"SET 0,D",    // c2
	"SET 0,E",    // c3
	"SET 0,H",    // c4
	"SET 0,L",    // c5
	"SET 0,(HL)", // c6
	"SET 0,A",    // c7
	"SET 1,B",    // c8
	"SET 1,C",    // c9
	"SET 1,D",    // ca
	"SET 1,E",    // cb
	"SET 1,H",    // cc
	"SET 1,L",    // cd
	"SET 1,(HL)", // ce
	"SET 1,A",    // cf
	"SET 2,B",    // d0
	"SET 2,C",    // d1
	"SET 2,D",    // d2
	"SET 2,E",    // d3
	"SET 2,H",    // d4
	"SET 2,L",    // d5
	"SET 2,(HL)", // d6
	"SET 2,A",    // d7
	"SET 3,B",    // d8
	"SET 3,C",    // d9
	"SET 3,D",    // da
	"SET 3,E",    // db
	"SET 3,H",    // dc
	"SET 3,L",    // dd
	"SET 3,(HL)", // de
	"SET 3,A",    // df
	"SET 4,B",    // e0
	"SET 4,C",    // e1
	"SET 4,D",    // e2
	"SET 4,E",    // e3
	"SET 4,H",    // e4
	"SET 4,L",    // e5
	"SET 4,(HL)", // e6
	"SET 4,A",    // e7
	"SET 5,B",    // e8
	"SET 5,C",    // e9
	"SET 5,D",    // ea
	"SET 5,E",    // eb
	"SET 5,H",    // ec
	"SET 5,L",    // ed
	"SET 5,(HL)", // ee
	"SET 5,A",    // ef
	"SET 6,B",    // f0
	"SET 6,C",    // f1
	"SET 6,D",    // f2
	"SET 6,E",    // f3
	"SET 6,H",    // f4
	"SET 6,L",    // f5
	"SET 6,(HL)", // f6
	"SET 6,A",    // f7
	"SET 7,B",    // f8
	"SET 7,C",    // f9
	"SET 7,D",    // fa
	"SET 7,E",    // fb
	"SET 7,H",    // fc
	"SET 7,L",    // fd
	"SET 7,(HL)", // fe
	"SET 7, A",   // ff
}
