all: borzgbc serialServer

serialServer: cmd/serial/serialServer.go
	go build cmd/serial/serialServer.go

borzgbc: cmd/sdl/borzgbc.go
	go build cmd/sdl/borzgbc.go

clean:
	rm -f borzgbc serialServer *.exe
