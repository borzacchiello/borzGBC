package gbc

func panicIfErr(e error) {
	if e != nil {
		panic(e)
	}
}
