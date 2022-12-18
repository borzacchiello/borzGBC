package gbc

const TIMER_INC_FREQ int = 16384

type Timer struct {
	GBC                 *Console
	DIV, TIMA, TMA, TAC uint8

	cpuCyclesCount uint64
	freq           int
}

func MakeTimer(c *Console) *Timer {
	t := &Timer{
		GBC:            c,
		freq:           TIMER_INC_FREQ,
		cpuCyclesCount: 0,
	}
	return t
}

func (t *Timer) Tick(cycles int) {
	t.cpuCyclesCount += uint64(cycles)

	t.DIV = uint8(t.cpuCyclesCount * uint64(t.freq) / uint64(t.GBC.CPUFreq))
	if t.cpuCyclesCount > 256*uint64(t.GBC.CPUFreq)/uint64(t.freq) {
		// Maximum count value
		t.cpuCyclesCount %= 256 * uint64(t.GBC.CPUFreq) / uint64(t.freq)
	}
}

func (t *Timer) Reset() {
	t.cpuCyclesCount = 0
	t.DIV = 0
}
