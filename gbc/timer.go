package gbc

import "encoding/gob"

const DIV_THRESHOLD int = 256

type Timer struct {
	GBC            *Console
	DIV            uint16
	TIMA, TMA, TAC uint8

	divCounter, timaCounter int
}

func (t *Timer) Save(encoder *gob.Encoder) {
	panicIfErr(encoder.Encode(t.DIV))
	panicIfErr(encoder.Encode(t.TIMA))
	panicIfErr(encoder.Encode(t.TMA))
	panicIfErr(encoder.Encode(t.TAC))
	panicIfErr(encoder.Encode(t.divCounter))
	panicIfErr(encoder.Encode(t.timaCounter))
}

func (t *Timer) Load(decoder *gob.Decoder) {
	panicIfErr(decoder.Decode(&t.DIV))
	panicIfErr(decoder.Decode(&t.TIMA))
	panicIfErr(decoder.Decode(&t.TMA))
	panicIfErr(decoder.Decode(&t.TAC))
	panicIfErr(decoder.Decode(&t.divCounter))
	panicIfErr(decoder.Decode(&t.timaCounter))
}

func MakeTimer(c *Console) *Timer {
	t := &Timer{
		GBC: c,
	}
	return t
}

func (t *Timer) reset() {
	t.divCounter = 0
	t.timaCounter = 0
	t.DIV = 0
	t.TIMA = t.TMA
}

func (t *Timer) updateDiv(ticks int) {
	t.divCounter += ticks * 4
	for t.divCounter >= DIV_THRESHOLD {
		t.divCounter -= DIV_THRESHOLD
		t.DIV += 1
	}
}

func (t *Timer) updateTima(ticks int) {
	if t.TAC&4 == 0 {
		return
	}
	t.timaCounter += ticks * 4

	threshold := 0
	switch t.TAC & 3 {
	case 0:
		threshold = 1024
	case 1:
		threshold = 16
	case 2:
		threshold = 64
	case 3:
		threshold = 256
	}

	for t.timaCounter >= threshold {
		t.timaCounter -= threshold
		if t.TIMA == 0xFF {
			t.TIMA = t.TMA
			t.GBC.CPU.SetInterrupt(InterruptTimer.Mask)
		} else {
			t.TIMA += 1
		}
	}
}

func (t *Timer) Tick(ticks int) {
	t.updateDiv(ticks)
	t.updateTima(ticks)
}
