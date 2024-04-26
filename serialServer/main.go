package main

import (
	"log"
	"net"
	"os"
)

func listenLoop(listen net.Listener) {
	log.Printf("waiting for connections!")

	conn1, err := listen.Accept()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	log.Printf("%s connected...", conn1.RemoteAddr().String())
	conn2, err := listen.Accept()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	log.Printf("%s connected...", conn2.RemoteAddr().String())

	log.Printf("both connected!")
	go func() {
		recvBuf := make([]byte, 1024)
		for {
			n, err := conn1.Read(recvBuf)
			if err != nil {
				break
			}
			_, err = conn2.Write(recvBuf[:n])
			if err != nil {
				break
			}
		}
		conn2.Close()
	}()

	recvBuf := make([]byte, 1024)
	for {
		n, err := conn2.Read(recvBuf)
		if err != nil {
			break
		}
		_, err = conn1.Write(recvBuf[:n])
		if err != nil {
			break
		}
	}
	conn1.Close()

	log.Printf("bye!")
}

func main() {
	listen, err := net.Listen("tcp", "127.0.0.1:31000")
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	defer listen.Close()

	for {
		listenLoop(listen)
	}
}
