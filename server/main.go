package main

import (
	"io"
	"log"
	"net"
	"os"
)

const (
	HOST = "localhost"
	PORT = "3333"
	TYPE = "tcp"
)

func main() {
	listen, err := net.Listen(TYPE, HOST+":"+PORT)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	// close listener
	defer listen.Close()
	for {
		conn1, err := listen.Accept()
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		conn2, err := listen.Accept()
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		go handleRequest(conn1, conn2)
	}
}

func handleRequest(conn1 net.Conn, conn2 net.Conn) {
	log.Printf("handling requests between %s and %s\n", conn1, conn2)

	go io.Copy(conn1, conn2)
	io.Copy(conn2, conn1)

	conn1.Close()
	conn2.Close()
}
