package gbc

type Console struct {
	Cart *Cart
	CPU  *GBCpu
}

func (cons *Console) Read(addr uint16) uint8 {
	switch {
	case 0x0000 <= addr && addr <= 0x3FFF:
		return cons.Cart.ROMBanks[0][addr]
	}
	return 0
}

func (cons *Console) Write(addr uint16, value uint8) {
	// TODO
}

func MakeConsole(rom_filepath string) (*Console, error) {
	cart, err := LoadCartridge(rom_filepath)
	if err != nil {
		return nil, err
	}

	res := &Console{
		Cart: cart,
		CPU:  &GBCpu{},
	}
	res.CPU.memory = res
	return res, nil
}
