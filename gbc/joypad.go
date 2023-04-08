package gbc

type JoypadState struct {
	A, B, UP, DOWN, LEFT, RIGHT, START, SELECT bool
	ActionSelector                             bool
	DirectionSelector                          bool
}

type Joypad struct {
	BackState  JoypadState
	FrontState JoypadState

	cons  *Console
	ticks int
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

func (j *JoypadState) PackButtons() uint8 {
	res := uint8(0x3f)

	if j.DirectionSelector {
		res &= ^uint8(16)
		if j.RIGHT {
			res &= ^uint8(1)
		}
		if j.LEFT {
			res &= ^uint8(2)
		}
		if j.UP {
			res &= ^uint8(4)
		}
		if j.DOWN {
			res &= ^uint8(8)
		}
	}
	if j.ActionSelector {
		res &= ^uint8(32)
		if j.A {
			res &= ^uint8(1)
		}
		if j.B {
			res &= ^uint8(2)
		}
		if j.SELECT {
			res &= ^uint8(4)
		}
		if j.START {
			res &= ^uint8(8)
		}
	}
	return res
}
