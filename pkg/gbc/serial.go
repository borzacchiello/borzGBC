package gbc

import (
	"encoding/gob"
)

var SERIAL_TICK_COUNT = 4096

type Serial struct {
	frontend      Frontend
	GBC           *Console
	SB            uint8
	SC            uint8
	serialCounter int
}

func (s *Serial) Save(encoder *gob.Encoder) {
	panicIfErr(encoder.Encode(s.serialCounter))
	panicIfErr(encoder.Encode(s.SB))
	panicIfErr(encoder.Encode(s.SC))
}

func (s *Serial) Load(decoder *gob.Decoder) error {
	errs := []error{
		decoder.Decode(&s.serialCounter),
		decoder.Decode(&s.SB),
		decoder.Decode(&s.SC),
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func MakeSerial(c *Console, frontend Frontend) *Serial {
	s := &Serial{
		GBC:           c,
		frontend:      frontend,
		serialCounter: 0,
	}
	return s
}

func (s *Serial) Tick(ticks int) {
	s.serialCounter += ticks * 4
	if s.serialCounter >= SERIAL_TICK_COUNT {
		s.serialCounter -= SERIAL_TICK_COUNT

		inSB, inSC := s.frontend.ExchangeSerial(s.SB, s.SC)
		shouldTriggerInterrupt := s.SC&0x81 == 0x81
		if inSC&0x80 == 0x80 && s.SC&0x80 == 0x80 && inSC&1 != s.SC&1 {
			shouldTriggerInterrupt = true
			s.SB = inSB
		}
		if shouldTriggerInterrupt {
			s.SC &= 1
			s.GBC.CPU.SetInterrupt(InterruptSerial.Mask)
		}
	}
}
