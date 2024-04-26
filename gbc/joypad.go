package gbc

import "encoding/gob"

type JoypadState struct {
	A, B, UP, DOWN, LEFT, RIGHT, START, SELECT bool
}

type Joypad struct {
	BackState  JoypadState
	FrontState JoypadState

	ActionSelector    bool
	DirectionSelector bool

	cons  *Console
	ticks int
}

func (j *Joypad) Save(encoder *gob.Encoder) {
	panicIfErr(encoder.Encode(j.BackState))
	panicIfErr(encoder.Encode(j.FrontState))
	panicIfErr(encoder.Encode(j.ActionSelector))
	panicIfErr(encoder.Encode(j.DirectionSelector))
	panicIfErr(encoder.Encode(j.ticks))
}

func (j *Joypad) Load(decoder *gob.Decoder) {
	panicIfErr(decoder.Decode(&j.BackState))
	panicIfErr(decoder.Decode(&j.FrontState))
	panicIfErr(decoder.Decode(&j.ActionSelector))
	panicIfErr(decoder.Decode(&j.DirectionSelector))
	panicIfErr(decoder.Decode(&j.ticks))
}

func MakeJoypad(cons *Console) *Joypad {
	return &Joypad{cons: cons, ticks: 0}
}

func (j *Joypad) Tick(ticks int) {
	j.ticks += ticks

	// ~ 10 ms
	thresholdTicks := j.cons.CPUFreq * 10 / 4000
	if j.ticks >= thresholdTicks {
		j.ticks -= thresholdTicks
		if j.FrontState != j.BackState {
			j.FrontState = j.BackState
			j.cons.CPU.SetInterrupt(InterruptJoypad.Mask)
		}
	}
}

func bto8(b bool) uint8 {
	if b {
		return uint8(1)
	}
	return uint8(0)
}

func itob(v uint8) bool {
	return v != 0
}

func (j *JoypadState) Serialize() uint8 {
	r := uint8(0)
	r |= bto8(j.A) << 0
	r |= bto8(j.B) << 1
	r |= bto8(j.DOWN) << 2
	r |= bto8(j.LEFT) << 3
	r |= bto8(j.RIGHT) << 4
	r |= bto8(j.UP) << 5
	r |= bto8(j.START) << 6
	r |= bto8(j.SELECT) << 7
	return r
}

func (j *JoypadState) Unserialize(v uint8) {
	j.A = itob(v & (1 << 0))
	j.B = itob(v & (1 << 1))
	j.DOWN = itob(v & (1 << 2))
	j.LEFT = itob(v & (1 << 3))
	j.RIGHT = itob(v & (1 << 4))
	j.UP = itob(v & (1 << 5))
	j.START = itob(v & (1 << 6))
	j.SELECT = itob(v & (1 << 7))
}

func (j *Joypad) PackButtons() uint8 {
	res := uint8(0x3f)

	if j.DirectionSelector {
		res &= ^uint8(16)
		if j.FrontState.RIGHT {
			res &= ^uint8(1)
		}
		if j.FrontState.LEFT {
			res &= ^uint8(2)
		}
		if j.FrontState.UP {
			res &= ^uint8(4)
		}
		if j.FrontState.DOWN {
			res &= ^uint8(8)
		}
	}
	if j.ActionSelector {
		res &= ^uint8(32)
		if j.FrontState.A {
			res &= ^uint8(1)
		}
		if j.FrontState.B {
			res &= ^uint8(2)
		}
		if j.FrontState.SELECT {
			res &= ^uint8(4)
		}
		if j.FrontState.START {
			res &= ^uint8(8)
		}
	}
	return res
}
