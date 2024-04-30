all: borzgbc serialServer

serialServer: cmd/serial/serialServer.go
	go build cmd/serial/serialServer.go

borzgbc: cmd/sdl/borzgbc.go
	go build cmd/sdl/borzgbc.go

wasm: cmd/wasm/borzgbc.go
	GOOS=js GOARCH=wasm go build -o web/assets/borzgbc.wasm cmd/wasm/borzgbc.go

clean:
	rm -f borzgbc serialServer web/assets/borzgbc.wasm *.exe
