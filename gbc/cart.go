package gbc

import (
	"encoding/binary"
	"os"
)

const HeaderSize = 0x4A

type Header struct {
	EntryCode        [4]uint8  // 0100h - 0104h
	NintendoLogo     [48]uint8 // 0104h - 0133h
	GameTitle        [11]uint8 // 0134h - 0143h Length of 16 in newer ROMS (ManufacturerCode and CgbFlag was included)
	ManufacturerCode [4]uint8  // 013Fh - 0142h Not present in old ROMS (within GameTitle)
	CgbFlag          uint8     // 0143h - 0143h Not present in old ROMS (within GameTitle)
	LicenseeCode     [2]uint8  // 0144h - 0145h
	SgbFlag          uint8     // 0146h - 0146h
	CartridgeType    uint8     // 0147h - 0147h
	RomSize          uint8     // 0148h - 0148h
	RamSize          uint8     // 0149h - 0149h
	DestinationCode  uint8     // 014Ah - 014Ah
	OldLicenseeCode  uint8     // 014Bh - 014Bh
	RomVersionNumber uint8     // 014Ch - 014Ch
	HeaderChecksum   uint8     // 014Dh - 014Dh
	GlobalChecksum   uint16    // 014Eh - 014Fh
}

type Cart struct {
	filepath string
	header   Header
	ROMBanks [][16384]uint8
	RAMBanks [][8192]uint8
}

type CartError string

func (err CartError) Error() string {
	return string(err)
}

func LoadCartridge(filepath string) (*Cart, error) {
	res := &Cart{}
	res.filepath = filepath

	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read Header
	_, err = f.Seek(0x0100, 0)
	if err != nil {
		return nil, err
	}
	err = binary.Read(f, binary.BigEndian, &res.header)
	if err != nil {
		return nil, err
	}

	// Read ROM Banks
	numROMBanks := 0
	if res.header.RomSize <= 8 {
		numROMBanks = 2 * (1 << int(res.header.RomSize))
	} else if res.header.RomSize == 52 {
		numROMBanks = 72
	} else if res.header.RomSize == 53 {
		numROMBanks = 80
	} else if res.header.RomSize == 54 {
		numROMBanks = 96
	} else {
		return nil, CartError("Invalid header.RomSize value")
	}

	res.ROMBanks = make([][16384]uint8, numROMBanks)
	_, err = f.Seek(0, 0)
	if err != nil {
		return nil, err
	}
	for i := 0; i < numROMBanks; i++ {
		num, err := f.Read(res.ROMBanks[i][:])
		if err != nil {
			return nil, err
		}
		if num != 16384 {
			return nil, CartError("Unable to read ROMBank: not enough data in file")
		}
	}

	// Create RAM Banks
	numRAMBanks := 0
	switch res.header.RamSize {
	case 0:
		numRAMBanks = 0
	case 2:
		numRAMBanks = 1
	case 3:
		numRAMBanks = 4
	case 4:
		numRAMBanks = 16
	case 5:
		numRAMBanks = 8
	default:
		return nil, CartError("Invalid header.RamSize")
	}

	res.RAMBanks = make([][8192]uint8, numRAMBanks)

	// Check if we read the whole file
	off, err := f.Seek(0, 1)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if fi.Size() != off {
		return nil, CartError("Unread data at the end of the cartridge")
	}

	return res, nil
}

func (cart *Cart) GetGameTitle() string {
	// FIXME: in older ROMS it should include also 5 bytes after the GameTitle array
	return string(cart.header.GameTitle[:])
}
