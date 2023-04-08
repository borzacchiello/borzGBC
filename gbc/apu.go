// MIT License

// Copyright (c) 2017 Humphrey Shotton

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// This APU implementation is taken from: https://github.com/Humpheh/goboy

package gbc

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
)

const (
	sampleRate           = 44100
	twoPi                = 2 * math.Pi
	perSample            = 1 / float64(sampleRate)
	maxFrameBufferLength = 5000
)

var sweepTimes = map[byte]float64{
	1: 7.8 / 1000,
	2: 15.6 / 1000,
	3: 23.4 / 1000,
	4: 31.3 / 1000,
	5: 39.1 / 1000,
	6: 46.9 / 1000,
	7: 54.7 / 1000,
}

var soundMask = []byte{
	/* 0xFF10 */ 0xFF, 0xC0, 0xFF, 0x00, 0x40,
	/* 0xFF15 */ 0x00, 0xC0, 0xFF, 0x00, 0x40,
	/* 0xFF1A */ 0x80, 0x00, 0x60, 0x00, 0x40,
	/* 0xFF20 */ 0x00, 0x3F, 0xFF, 0xFF, 0x40,
	/* 0xFF24 */ 0xFF, 0xFF, 0x80,
}

var channel3Volume = map[byte]float64{0: 0, 1: 1, 2: 0.5, 3: 0.25}

var squareLimits = map[byte]float64{
	0: -0.25, // 12.5% ( _-------_-------_------- )
	1: -0.5,  // 25%   ( __------__------__------ )
	2: 0,     // 50%   ( ____----____----____---- ) (normal)
	3: 0.5,   // 75%   ( ______--______--______-- )
}

// WaveGenerator is a function which can be used for generating waveform
// samples for different channels.
type WaveGenerator func(t float64) int8

// Channel represents one of four Gameboy sound channels.
type Channel struct {
	frequency float64
	generator WaveGenerator
	time      float64
	amplitude float64

	// Duration in samples
	duration int
	length   int

	envelopeVolume     int
	envelopeTime       int
	envelopeSteps      int
	envelopeStepsInit  int
	envelopeSamples    int
	envelopeIncreasing bool

	sweepTime     float64
	sweepStepLen  byte
	sweepSteps    byte
	sweepStep     byte
	sweepIncrease bool

	onL bool
	onR bool
	// Debug flag to turn off sound output
	debugOff bool
}

// APU is the GameBoy's audio processing unit. Audio comprises four
// channels, each one controlled by a set of registers.
//
// Channels 1 and 2 are both Square channels, channel 3 is a arbitrary
// waveform channel which can be set in RAM, and channel 4 outputs noise.
type Apu struct {
	frontend     Frontend
	GBC          *Console
	globalVolume float64
	playing      bool

	memory      [256]byte
	waveformRam [32]byte

	chn1, chn2, chn3, chn4 *Channel
	tickCounter            float64
	lVol, rVol             float64
}

func MakeApu(GBC *Console, frontend Frontend) *Apu {
	a := &Apu{}

	a.GBC = GBC
	a.globalVolume = 0.5
	a.playing = true
	a.frontend = frontend
	a.lVol = 1
	a.rVol = 1

	// Sets waveform ram to:
	// 00 0F 00 0F  00 0F 00 0F  00 0F 00 0F  00 0F 00 0F
	for x := 0x0; x < 0x20; x++ {
		if x&2 == 0 {
			a.waveformRam[x] = 0x00
		} else {
			a.waveformRam[x] = 0x0F
		}
	}

	// Create the channels with their sounds
	a.chn1 = &Channel{debugOff: false}
	a.chn2 = &Channel{debugOff: false}
	a.chn3 = &Channel{debugOff: false}
	a.chn4 = &Channel{debugOff: false}
	return a
}

func (a *Apu) ToggleAudio() {
	a.playing = !a.playing
}

func (a *Apu) IncreaseAudio() {
	a.globalVolume += 0.05
	if a.globalVolume > 1.0 {
		a.globalVolume = 1
	}
}

func (a *Apu) DecreaseAudio() {
	a.globalVolume -= 0.05
	if a.globalVolume < 0 {
		a.globalVolume = 0
	}
}

func (a *Apu) Tick(cpuTicks int) {
	cpuTicks *= 4
	if !a.playing {
		return
	}
	increment := float64(cpuTicks)
	if a.GBC.DoubleSpeedMode {
		increment /= 2
	}
	a.tickCounter += increment

	cpuTicksPerSample := float64(a.GBC.CPUFreq) / sampleRate * 0.99
	if a.tickCounter < cpuTicksPerSample {
		return
	}
	a.tickCounter -= cpuTicksPerSample

	chn1l, chn1r := a.chn1.Sample()
	chn2l, chn2r := a.chn2.Sample()
	chn3l, chn3r := a.chn3.Sample()
	chn4l, chn4r := a.chn4.Sample()

	valL := (chn1l + chn2l + chn3l + chn4l) / 4
	valR := (chn1r + chn2r + chn3r + chn4r) / 4

	// fmt.Printf("valL: %f, valR: %f\n", float64(valL)*a.lVol, float64(valR)*a.rVol)

	a.frontend.NotifyAudioSample(
		int8(float64(valL)*a.lVol*a.globalVolume),
		int8(float64(valR)*a.rVol*a.globalVolume))
}

// Read returns a value from the APU.
func (a *Apu) Read(address uint16) byte {
	if address >= 0xFF30 {
		return a.waveformRam[address-0xFF30]
	}
	// TODO: we should modify the sound memory as we're sampling
	return a.memory[address-0xFF00] & soundMask[address-0xFF10]
}

// Write a value to the APU registers.
func (a *Apu) Write(address uint16, value byte) {
	a.memory[address-0xFF00] = value

	if 0xFF30 <= address && address <= 0xFF3F {
		a.writeWaveform(address, value)
		return
	}

	switch address {
	// Channel 1
	case 0xFF10:
		// -PPP NSSS Sweep period, negate, shift
		a.chn1.sweepStepLen = (a.memory[0x10] & 0b111_0000) >> 4
		a.chn1.sweepSteps = a.memory[0x10] & 0b111
		a.chn1.sweepIncrease = a.memory[0x10]&0b1000 == 0 // 1 = decrease
	case 0xFF11:
		// DDLL LLLL Duty, Length load
		duty := (value & 0b1100_0000) >> 6
		a.chn1.generator = Square(squareLimits[duty])
		a.chn1.length = int(value & 0b0011_1111)
	case 0xFF12:
		// VVVV APPP - Starting volume, Envelop add mode, period
		envVolume, envDirection, envSweep := a.extractEnvelope(value)
		a.chn1.envelopeVolume = int(envVolume)
		a.chn1.envelopeSamples = int(envSweep) * sampleRate / 64
		a.chn1.envelopeIncreasing = envDirection == 1
	case 0xFF13:
		// FFFF FFFF Frequency LSB
		frequencyValue := uint16(a.memory[0x14]&0b111)<<8 | uint16(value)
		a.chn1.frequency = 131072 / (2048 - float64(frequencyValue))
	case 0xFF14:
		// TL-- -FFF Trigger, Length Enable, Frequencu MSB
		frequencyValue := uint16(value&0b111)<<8 | uint16(a.memory[0x13])
		a.chn1.frequency = 131072 / (2048 - float64(frequencyValue))
		if value&0b1000_0000 != 0 {
			if a.chn1.length == 0 {
				a.chn1.length = 64
			}
			duration := -1
			if value&0b100_0000 != 0 { // 1 = use length
				duration = int(float64(a.chn1.length)*(1.0/64)) * sampleRate
			}
			a.chn1.Reset(duration)
			a.chn1.envelopeSteps = a.chn1.envelopeVolume
			a.chn1.envelopeStepsInit = a.chn1.envelopeVolume
			// TODO: Square 1's sweep does several things (see frequency sweep).
		}

	// Channel 2
	case 0xFF15:
		// ---- ---- Not used
	case 0xFF16:
		// DDLL LLLL Duty, Length load (64-L)
		pattern := (value & 0b1100_0000) >> 6
		a.chn2.generator = Square(squareLimits[pattern])
		a.chn2.length = int(value & 0b11_1111)
	case 0xFF17:
		// VVVV APPP Starting volume, Envelope add mode, period
		envVolume, envDirection, envSweep := a.extractEnvelope(value)
		a.chn2.envelopeVolume = int(envVolume)
		a.chn2.envelopeSamples = int(envSweep) * sampleRate / 64
		a.chn2.envelopeIncreasing = envDirection == 1
	case 0xFF18:
		// FFFF FFFF Frequency LSB
		frequencyValue := uint16(a.memory[0x19]&0b111)<<8 | uint16(value)
		a.chn2.frequency = 131072 / (2048 - float64(frequencyValue))
	case 0xFF19:
		// TL-- -FFF Trigger, Length enable, Frequency MSB
		if value&0b1000_0000 != 0 {
			if a.chn2.length == 0 {
				a.chn2.length = 64
			}
			duration := -1
			if value&0b100_0000 != 0 {
				duration = int(float64(a.chn2.length)*(1.0/64)) * sampleRate
			}
			a.chn2.Reset(duration)
			a.chn2.envelopeSteps = a.chn2.envelopeVolume
			a.chn2.envelopeStepsInit = a.chn2.envelopeVolume
		}
		frequencyValue := uint16(value&0b111)<<8 | uint16(a.memory[0x18])
		a.chn2.frequency = 131072 / (2048 - float64(frequencyValue))

	// Channel 3
	case 0xFF1A:
		// E--- ---- DAC power
		a.chn3.envelopeStepsInit = int((value & 0b1000_0000) >> 7)
	case 0xFF1B:
		// LLLL LLLL Length load
		a.chn3.length = int(value)
	case 0xFF1C:
		// -VV- ---- Volume code
		selection := (value & 0b110_0000) >> 5
		a.chn3.amplitude = channel3Volume[selection]
	case 0xFF1D:
		// FFFF FFFF Frequency LSB
		frequencyValue := uint16(a.memory[0x1E]&0b111)<<8 | uint16(value)
		a.chn3.frequency = 65536 / (2048 - float64(frequencyValue))
	case 0xFF1E:
		// TL-- -FFF Trigger, Length enable, Frequency MSB
		if value&0b1000_0000 != 0 {
			if a.chn3.length == 0 {
				a.chn3.length = 256
			}
			duration := -1
			if value&0b100_0000 != 0 { // 1 = use length
				duration = int((256-float64(a.chn3.length))*(1.0/256)) * sampleRate
			}
			a.chn3.generator = Waveform(func(i int) int8 { return int8(a.waveformRam[i]) })
			a.chn3.duration = duration
		}
		frequencyValue := uint16(value&0b111)<<8 | uint16(a.memory[0x1D])
		a.chn3.frequency = 65536 / (2048 - float64(frequencyValue))

	// Channel 4
	case 0xFF1F:
		// ---- ---- Not used
	case 0xFF20:
		// --LL LLLL Length load
		a.chn4.length = int(value & 0b11_1111)
	case 0xFF21:
		// VVVV APPP Starting volume, Envelope add mode, period
		envVolume, envDirection, envSweep := a.extractEnvelope(value)
		a.chn4.envelopeVolume = int(envVolume)
		a.chn4.envelopeSamples = int(envSweep) * sampleRate / 64
		a.chn4.envelopeIncreasing = envDirection == 1
	case 0xFF22:
		// SSSS WDDD Clock shift, Width mode of LFSR, Divisor code
		shiftClock := float64((value & 0b1111_0000) >> 4)
		// TODO: counter step width
		divRatio := float64(value & 0b111)
		if divRatio == 0 {
			divRatio = 0.5
		}
		a.chn4.frequency = 524288 / divRatio / math.Pow(2, shiftClock+1)
	case 0xFF23:
		// TL-- ---- Trigger, Length enable
		if value&0x80 == 0x80 {
			duration := -1
			if value&0b100_0000 != 0 { // 1 = use length
				duration = int(float64(61-a.chn4.length)*(1.0/256)) * sampleRate
			}
			a.chn4.generator = Noise()
			a.chn4.Reset(duration)
			a.chn4.envelopeSteps = a.chn4.envelopeVolume
			a.chn4.envelopeStepsInit = a.chn4.envelopeVolume
		}

	case 0xFF24:
		// Volume control
		a.lVol = float64((value&0x70)>>4) / 7
		a.rVol = float64(value&0x7) / 7

	case 0xFF25:
		// Channel control
		a.chn1.onR = value&0x1 != 0
		a.chn2.onR = value&0x2 != 0
		a.chn3.onR = value&0x4 != 0
		a.chn4.onR = value&0x8 != 0
		a.chn1.onL = value&0x10 != 0
		a.chn2.onL = value&0x20 != 0
		a.chn3.onL = value&0x40 != 0
		a.chn4.onL = value&0x80 != 0
	}
	// TODO: if writing to FF26 bit 7 destroy all contents (also cannot access)
}

// WriteWaveform writes a value to the waveform ram.
func (a *Apu) writeWaveform(address uint16, value byte) {
	soundIndex := (address - 0xFF30) * 2
	a.waveformRam[soundIndex] = (value >> 4) & 0xF
	a.waveformRam[soundIndex+1] = value & 0xF
}

// ToggleSoundChannel toggles a sound channel for debugging.
func (a *Apu) ToggleSoundChannel(channel int) {
	switch channel {
	case 1:
		a.chn1.debugOff = !a.chn1.debugOff
	case 2:
		a.chn2.debugOff = !a.chn2.debugOff
	case 3:
		a.chn3.debugOff = !a.chn3.debugOff
	case 4:
		a.chn4.debugOff = !a.chn4.debugOff
	}
}

func (a *Apu) IsMuted() bool {
	return !a.playing
}

func (a *Apu) IsChMuted(n int) bool {
	switch n {
	case 1:
		return a.chn1.debugOff
	case 2:
		return a.chn2.debugOff
	case 3:
		return a.chn3.debugOff
	case 4:
		return a.chn4.debugOff
	}
	return false
}

func (a *Apu) GetVolumeString() string {
	return fmt.Sprintf("volume: %02d / 100", int(a.globalVolume*100))
}

func (a *Apu) GetSoundChannelsString() string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "ch1: ")
	if a.chn1.debugOff {
		fmt.Fprintf(&builder, "ON  ")
	} else {
		fmt.Fprintf(&builder, "OFF ")
	}
	fmt.Fprintf(&builder, "ch2: ")
	if a.chn2.debugOff {
		fmt.Fprintf(&builder, "ON  ")
	} else {
		fmt.Fprintf(&builder, "OFF ")
	}
	fmt.Fprintf(&builder, "ch3: ")
	if a.chn3.debugOff {
		fmt.Fprintf(&builder, "ON  ")
	} else {
		fmt.Fprintf(&builder, "OFF ")
	}
	fmt.Fprintf(&builder, "ch4: ")
	if a.chn4.debugOff {
		fmt.Fprintf(&builder, "ON  ")
	} else {
		fmt.Fprintf(&builder, "OFF ")
	}
	return builder.String()
}

// Extract some envelope variables from a byte.
func (a *Apu) extractEnvelope(val byte) (volume, direction, sweep byte) {
	volume = (val & 0xF0) >> 4
	direction = (val & 0x8) >> 3 // 1 or 0
	sweep = val & 0x7
	return
}

// Square returns a square wave generator with a given mod. This is used
// for channels 1 and 2.
func Square(mod float64) WaveGenerator {
	return func(t float64) int8 {
		if math.Sin(t) <= mod {
			return 64
		}
		return 0
	}
}

// Waveform returns a wave generator for some waveform ram. This is used
// by channel 3.
func Waveform(ram func(i int) int8) WaveGenerator {
	return func(t float64) int8 {
		idx := int(math.Floor(t/twoPi*32)) % 0x20
		return ram(idx) << 2
	}
}

// Noise returns a wave generator for a noise channel. This is used by
// channel 4.
func Noise() WaveGenerator {
	var last float64
	var val int8
	return func(t float64) int8 {
		if t-last > twoPi {
			last = t
			val = int8(rand.Intn(32))
		}
		return int8(val)
	}
}

// Sample returns a single sample for streaming the sound output. Each sample
// will increase the internal timer based on the global sample rate.
func (chn *Channel) Sample() (outputL, outputR uint16) {
	var output uint16
	step := chn.frequency * twoPi / float64(sampleRate)
	chn.time += step
	if chn.shouldPlay() {
		// Take the sample value from the generator
		if !chn.debugOff {
			output = uint16(float64(chn.generator(chn.time)) * chn.amplitude)
		}
		if chn.duration > 0 {
			chn.duration--
		}
	}
	chn.updateEnvelope()
	chn.updateSweep()
	if chn.onL {
		outputL = output
	}
	if chn.onR {
		outputR = output
	}
	return
}

// Reset the channel to some default variables for the sweep, amplitude,
// envelope and duration.
func (chn *Channel) Reset(duration int) {
	chn.amplitude = 1
	chn.envelopeTime = 0
	chn.sweepTime = 0
	chn.sweepStep = 0
	chn.duration = duration
}

// Returns if the channel should be playing or not.
func (chn *Channel) shouldPlay() bool {
	return (chn.duration == -1 || chn.duration > 0) &&
		chn.generator != nil && chn.envelopeStepsInit > 0
}

// Update the state of the channels envelope.
func (chn *Channel) updateEnvelope() {
	if chn.envelopeSamples > 0 {
		chn.envelopeTime += 1
		if chn.envelopeSteps > 0 && chn.envelopeTime >= chn.envelopeSamples {
			chn.envelopeTime -= chn.envelopeSamples
			chn.envelopeSteps--
			if chn.envelopeSteps == 0 {
				chn.amplitude = 0
			} else if chn.envelopeIncreasing {
				chn.amplitude = 1 - float64(chn.envelopeSteps)/float64(chn.envelopeStepsInit)
			} else {
				chn.amplitude = float64(chn.envelopeSteps) / float64(chn.envelopeStepsInit)
			}
		}
	}
}

// Update the state of the channels sweep.
func (chn *Channel) updateSweep() {
	if chn.sweepStep < chn.sweepSteps {
		t := sweepTimes[chn.sweepStepLen]
		chn.sweepTime += perSample
		if chn.sweepTime > t {
			chn.sweepTime -= t
			chn.sweepStep += 1

			if chn.sweepIncrease {
				chn.frequency += chn.frequency / math.Pow(2, float64(chn.sweepStep))
			} else {
				chn.frequency -= chn.frequency / math.Pow(2, float64(chn.sweepStep))
			}
		}
	}
}
