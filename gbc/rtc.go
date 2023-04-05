package gbc

import (
	"encoding/json"
	"fmt"
	"time"
)

type RTC struct {
	BaseTime, HaltTime int64
	Seconds            uint8 // $08 RTC S   Seconds   0-59 ($00-$3B)
	Minutes            uint8 // $09 RTC M   Minutes   0-59 ($00-$3B)
	Hours              uint8 // $0A RTC H   Hours     0-23 ($00-$17)
	DaysL              uint8 // $0B RTC DL  Lower 8 bits of Day Counter ($00-$FF)
	DaysH              uint8 // $0C RTC DH  Upper 1 bit of Day Counter, Carry Bit, Halt Flag
	//                  Bit 0  Most significant bit of Day Counter (Bit 8)
	//                  Bit 6  Halt (0=Active, 1=Stop Timer)
	//                  Bit 7  Day Counter Carry Bit (1=Counter Overflow)
}

func MakeRTC() RTC {
	res := RTC{
		BaseTime: time.Now().Unix(),
		DaysH:    64,
	}
	res.SyncTime()
	return res
}

func (rtc *RTC) String() string {
	return fmt.Sprintf("S: %d, M: %d, H: %d, D: %d, S: 0x%02x\n",
		rtc.Seconds, rtc.Minutes, rtc.Hours, rtc.DaysL, rtc.DaysH)
}

func (rtc *RTC) IsHalted() bool {
	return rtc.DaysH&64 > 0
}

func (rtc *RTC) getTime() int64 {
	if rtc.IsHalted() {
		return rtc.HaltTime
	}
	return time.Now().Unix()
}

func (rtc *RTC) recomputeBaseDate() {
	base := rtc.getTime()

	days := int64(rtc.DaysL)
	days += int64(rtc.DaysH&1) * 0x100
	days += int64((rtc.DaysH>>7)&1) * 0x200

	base -= days * 60 * 60 * 24
	base -= int64(rtc.Hours) * 60 * 60
	base -= int64(rtc.Minutes) * 60
	base -= int64(rtc.Seconds)

	rtc.BaseTime = base
}

func (rtc *RTC) SyncTime() {
	now := rtc.getTime()
	if now >= rtc.BaseTime {
		now = now - rtc.BaseTime
	} else {
		// the base time is in the future
		// let's fix it
		rtc.BaseTime = now
		now = 0
	}

	rtc.Seconds = uint8(now % 60)
	now /= 60
	rtc.Minutes = uint8(now % 60)
	now /= 60
	rtc.Hours = uint8(now % 24)
	now /= 24
	rtc.DaysL = uint8(now & 0xff)
	rtc.DaysH &= 0x40
	rtc.DaysH |= uint8((now >> 8) & 1)
	if now > 0x1ff {
		rtc.DaysH |= 0x80
	}
}

func (rtc *RTC) SetReg(addr uint8, value uint8) {
	// fmt.Printf("setting RTC reg %02x value %02x\n", addr, value)
	wasHalted := rtc.IsHalted()
	switch addr {
	case 0x08:
		rtc.Seconds = value
	case 0x09:
		rtc.Minutes = value
	case 0x0A:
		rtc.Hours = value
	case 0x0B:
		rtc.DaysL = value
	case 0x0C:
		rtc.DaysH = value
		if !wasHalted && rtc.IsHalted() {
			rtc.HaltTime = time.Now().Unix()
		}
	default:
		fmt.Printf("unexpected write to RTC @ 0x%02x <- %02x\n", addr, value)
	}

	rtc.recomputeBaseDate()
}

func (rtc *RTC) GetReg(addr uint8) uint8 {
	// fmt.Printf("reading RTC reg %02x\n   %s\n", addr, rtc)
	switch addr {
	case 0x08:
		return rtc.Seconds
	case 0x09:
		return rtc.Minutes
	case 0x0A:
		return rtc.Hours
	case 0x0B:
		return rtc.DaysL
	case 0x0C:
		return rtc.DaysH
	default:
		fmt.Printf("unexpected read from RTC @ 0x%02x\n", addr)
	}
	return 0xFF
}

func (rtc *RTC) Marshal() ([]byte, error) {
	return json.Marshal(rtc)
}

func (rtc *RTC) UnMarshal(data []byte) error {
	err := json.Unmarshal(data, rtc)
	if err != nil {
		return err
	}
	return nil
}
