package gbc

type Mapper interface {
	MapperRead(addr uint16) uint8
	MapperWrite(addr uint16, value uint8)
}
