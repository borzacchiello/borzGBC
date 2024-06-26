package gbc

import (
	"encoding/gob"
	"fmt"
)

type Mapper interface {
	MapperRead(addr uint16) uint8
	MapperWrite(addr uint16, value uint8)
	MapperSave(encoder *gob.Encoder)
	MapperLoad(decoder *gob.Decoder) error
}

func calculateMask(value uint) uint {
	if value == 0 {
		return 0
	}

	mask := uint(0)
	curr := value - 1
	for curr != 0 {
		mask <<= 1
		mask |= 1
		curr >>= 1
	}

	return mask
}

type ROMOnlyMapper struct {
	cart *Cart
}

func (m ROMOnlyMapper) MapperSave(encoder *gob.Encoder)       {}
func (m ROMOnlyMapper) MapperLoad(decoder *gob.Decoder) error { return nil }

func (m ROMOnlyMapper) MapperRead(addr uint16) uint8 {
	bank_n := addr >> 14
	return m.cart.ROMBanks[bank_n][addr&0x3fff]
}

func (m ROMOnlyMapper) MapperWrite(addr uint16, value uint8) {
	fmt.Printf("Trying to write on ROMOnlyMapper @ 0x%04x <- 0x%02x\n", addr, value)
}

type MBC1Mapper struct {
	cart     *Cart
	bankMask uint8
	ramMask  uint8

	ramEnabled     bool  // 0000 - 1FFF
	romBank        uint8 // 2000 - 3FFF
	ramBank        uint8 // 4000 - 5FFF
	advBankingMode bool  // 6000 - 7FFF
}

func (m *MBC1Mapper) MapperSave(encoder *gob.Encoder) {
	panicIfErr(encoder.Encode(m.bankMask))
	panicIfErr(encoder.Encode(m.ramMask))
	panicIfErr(encoder.Encode(m.ramEnabled))
	panicIfErr(encoder.Encode(m.romBank))
	panicIfErr(encoder.Encode(m.ramBank))
	panicIfErr(encoder.Encode(m.advBankingMode))
}

func (m *MBC1Mapper) MapperLoad(decoder *gob.Decoder) error {
	errs := []error{
		decoder.Decode(&m.bankMask),
		decoder.Decode(&m.ramMask),
		decoder.Decode(&m.ramEnabled),
		decoder.Decode(&m.romBank),
		decoder.Decode(&m.ramBank),
		decoder.Decode(&m.advBankingMode),
	}

	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func MakeMBC1Mapper(cart *Cart) *MBC1Mapper {
	return &MBC1Mapper{
		cart:           cart,
		bankMask:       uint8(calculateMask(uint(len(cart.ROMBanks)))),
		ramMask:        uint8(calculateMask(uint(len(cart.RAMBanks)))),
		ramEnabled:     false,
		romBank:        1,
		ramBank:        0,
		advBankingMode: false,
	}
}

func (m *MBC1Mapper) MapperRead(addr uint16) uint8 {
	switch {
	case addr <= 0x3FFF:
		if !m.advBankingMode {
			return m.cart.ROMBanks[0][addr]
		}
		bankOff := int(m.ramBank) << 5
		bankOff &= int(m.bankMask)
		return m.cart.ROMBanks[bankOff][addr]
	case 0x4000 <= addr && addr <= 0x7FFF:
		off := addr & 0x3FFF
		bank := int(m.romBank) | int(m.ramBank<<5)
		bank &= int(m.bankMask)
		return m.cart.ROMBanks[bank][off]
	case 0xA000 <= addr && addr <= 0xBFFF:
		if len(m.cart.RAMBanks) == 0 || !m.ramEnabled {
			return 0xFF
		}
		off := addr & 0x1FFF
		if !m.advBankingMode {
			return m.cart.RAMBanks[0][off]
		}
		return m.cart.RAMBanks[m.ramBank&m.ramMask][off]
	}

	fmt.Printf("Unexpected address in MBC1Mapper Read: 0x%04x\n", addr)
	return 0
}

func (m *MBC1Mapper) MapperWrite(addr uint16, value uint8) {
	switch {
	case addr <= 0x1FFF:
		if value&0xF == 0xA {
			m.ramEnabled = true
		} else {
			m.ramEnabled = false
		}
		return
	case 0x2000 <= addr && addr <= 0x3FFF:
		m.romBank = value & 0x1f
		if m.romBank == 0 {
			m.romBank = 1
		}
		return
	case 0x4000 <= addr && addr <= 0x5FFF:
		m.ramBank = value & 3
		return
	case 0x6000 <= addr && addr <= 0x7FFF:
		if value&1 == 0 {
			m.advBankingMode = false
		} else {
			m.advBankingMode = true
		}
		return
	case 0xA000 <= addr && addr <= 0xBFFF:
		if !m.ramEnabled || len(m.cart.RAMBanks) == 0 {
			return
		}
		off := addr & 0x1FFF
		if !m.advBankingMode {
			m.cart.RAMBanks[0][off] = value
		} else {
			m.cart.RAMBanks[m.ramBank&m.ramMask][off] = value
		}
		return
	}

	fmt.Printf("Unexpected address in MBC1Mapper Write: 0x%04x <- 0x%02x\n", addr, value)
}

type MBC3Mapper struct {
	cart *Cart

	rtcMapped    bool
	rtcRegVal    uint8
	rtcLastLatch uint8
	rtc          RTC

	ramEnabled bool   // 0000 - 1FFF
	rtcEnabled bool   // 0000 - 1FFF
	romBank    uint16 // 2000 - 3FFF
	ramBank    uint8  // 4000 - 5FFF
}

func (m *MBC3Mapper) MapperSave(encoder *gob.Encoder) {
	panicIfErr(encoder.Encode(m.rtcMapped))
	panicIfErr(encoder.Encode(m.rtcRegVal))
	panicIfErr(encoder.Encode(m.rtcLastLatch))
	panicIfErr(encoder.Encode(m.ramEnabled))
	panicIfErr(encoder.Encode(m.rtcEnabled))
	panicIfErr(encoder.Encode(m.romBank))
	panicIfErr(encoder.Encode(m.ramBank))
	m.rtc.Save(encoder)
}

func (m *MBC3Mapper) MapperLoad(decoder *gob.Decoder) error {
	errs := []error{
		decoder.Decode(&m.rtcMapped),
		decoder.Decode(&m.rtcRegVal),
		decoder.Decode(&m.rtcLastLatch),
		decoder.Decode(&m.ramEnabled),
		decoder.Decode(&m.rtcEnabled),
		decoder.Decode(&m.romBank),
		decoder.Decode(&m.ramBank),
		m.rtc.Load(decoder),
	}

	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func MakeMBC3Mapper(cart *Cart) *MBC3Mapper {
	res := &MBC3Mapper{
		cart:       cart,
		rtcMapped:  false,
		rtcRegVal:  0x08,
		ramEnabled: false,
		romBank:    1,
		ramBank:    0,
		rtc:        MakeRTC(),
	}
	return res
}

func (m *MBC3Mapper) MapperRead(addr uint16) uint8 {
	switch {
	case addr <= 0x3FFF:
		return m.cart.ROMBanks[0][addr]
	case 0x4000 <= addr && addr <= 0x7FFF:
		off := addr & 0x3FFF
		bank := int(m.romBank)
		return m.cart.ROMBanks[bank][off]
	case 0xA000 <= addr && addr <= 0xBFFF:
		if !m.rtcMapped {
			if !m.ramEnabled || len(m.cart.RAMBanks) == 0 {
				return 0x00
			}
			off := addr & 0x1FFF
			return m.cart.RAMBanks[m.ramBank][off]
		}
		return m.rtc.GetReg(m.rtcRegVal)
	}

	fmt.Printf("Unexpected address in MBC5Mapper Read: 0x%04x\n", addr)
	return 0
}

func (m *MBC3Mapper) MapperWrite(addr uint16, value uint8) {
	switch {
	case addr <= 0x1FFF:
		if value&0xF == 0xA {
			m.ramEnabled = true
			m.rtcEnabled = true
		} else {
			m.ramEnabled = false
			m.rtcEnabled = false
		}
		return
	case 0x2000 <= addr && addr <= 0x3FFF:
		m.romBank = uint16(value) & 0x7F
		if m.romBank == 0 {
			m.romBank = 1
		}
		m.romBank %= uint16(len(m.cart.ROMBanks))
		return
	case 0x4000 <= addr && addr <= 0x5FFF:
		if value <= 0x3 {
			m.rtcMapped = false
			m.ramBank = value
			m.ramBank %= uint8(len(m.cart.RAMBanks))
		} else if 0x8 <= value && value <= 0xC {
			m.rtcMapped = true
			m.rtcRegVal = value
		}
		return
	case 0x6000 <= addr && addr <= 0x7FFF:
		if m.rtcLastLatch == 0 && value == 1 {
			m.rtc.SyncTime()
		}
		m.rtcLastLatch = value
		return
	case 0xA000 <= addr && addr <= 0xBFFF:
		if !m.rtcMapped {
			if !m.ramEnabled || len(m.cart.RAMBanks) == 0 {
				return
			}
			off := addr & 0x1FFF
			m.cart.RAMBanks[m.ramBank][off] = value
		}
		m.rtc.SetReg(m.rtcRegVal, value)
		return
	}

	fmt.Printf("Unexpected address in MBC3Mapper Write: 0x%04x <- 0x%02x\n", addr, value)
}

type MBC5Mapper struct {
	cart *Cart

	ramEnabled bool   // 0000 - 1FFF
	romBank    uint16 // 2000 - 3FFF
	ramBank    uint8  // 4000 - 5FFF
}

func (m *MBC5Mapper) MapperSave(encoder *gob.Encoder) {
	panicIfErr(encoder.Encode(m.ramEnabled))
	panicIfErr(encoder.Encode(m.romBank))
	panicIfErr(encoder.Encode(m.ramBank))
}

func (m *MBC5Mapper) MapperLoad(decoder *gob.Decoder) error {
	errs := []error{
		decoder.Decode(&m.ramEnabled),
		decoder.Decode(&m.romBank),
		decoder.Decode(&m.ramBank),
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func MakeMBC5Mapper(cart *Cart) *MBC5Mapper {
	return &MBC5Mapper{
		cart:       cart,
		ramEnabled: false,
		romBank:    1,
		ramBank:    0,
	}
}

func (m *MBC5Mapper) MapperRead(addr uint16) uint8 {
	switch {
	case addr <= 0x3FFF:
		return m.cart.ROMBanks[0][addr]
	case 0x4000 <= addr && addr <= 0x7FFF:
		off := addr & 0x3FFF
		bank := int(m.romBank)
		return m.cart.ROMBanks[bank][off]
	case 0xA000 <= addr && addr <= 0xBFFF:
		if !m.ramEnabled || len(m.cart.RAMBanks) == 0 {
			return 0x00
		}
		off := addr & 0x1FFF
		return m.cart.RAMBanks[m.ramBank][off]
	}

	fmt.Printf("Unexpected address in MBC5Mapper Read: 0x%04x\n", addr)
	return 0
}

func (m *MBC5Mapper) MapperWrite(addr uint16, value uint8) {
	switch {
	case addr <= 0x1FFF:
		if value&0xF == 0xA {
			m.ramEnabled = true
		} else {
			m.ramEnabled = false
		}
		return
	case 0x2000 <= addr && addr <= 0x2FFF:
		m.romBank = (m.romBank & 0x100) | uint16(value)
		m.romBank %= uint16(len(m.cart.ROMBanks))
		return
	case 0x3000 <= addr && addr <= 0x3FFF:
		m.romBank = (m.romBank & 0xFF) | (uint16((value & 1)) << 8)
		m.romBank %= uint16(len(m.cart.ROMBanks))
		return
	case 0x4000 <= addr && addr <= 0x5FFF:
		if len(m.cart.RAMBanks) == 0 {
			return
		}
		m.ramBank = value & 0xf
		m.ramBank %= uint8(len(m.cart.RAMBanks))
		return
	case 0xA000 <= addr && addr <= 0xBFFF:
		if !m.ramEnabled || len(m.cart.RAMBanks) == 0 {
			return
		}
		off := addr & 0x1FFF
		m.cart.RAMBanks[m.ramBank][off] = value
		return
	}

	fmt.Printf("Unexpected address in MBC5Mapper Write: 0x%04x <- 0x%02x\n", addr, value)
}
