package gbc

import "fmt"

type Mapper interface {
	MapperRead(addr uint16) uint8
	MapperWrite(addr uint16, value uint8)
}

type ROMOnlyMapper struct {
	cart *Cart
}

func (m ROMOnlyMapper) MapperRead(addr uint16) uint8 {
	bank_n := addr >> 14
	return m.cart.ROMBanks[bank_n][addr&0x3fff]
}

func (m ROMOnlyMapper) MapperWrite(addr uint16, value uint8) {
	fmt.Printf("Trying to write on ROMOnlyMapper @ 0x%04x <- 0x%02x\n", addr, value)
	return
}
