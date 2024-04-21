package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	listen, err := net.Listen("tcp", "127.0.0.1:31000")
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	// close listener
	defer listen.Close()

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

	log.Printf("Both connections, ready to dance!")
	recvBuf1 := make([]byte, 2)
	recvBuf2 := make([]byte, 2)
	for {
		_, err := conn1.Read(recvBuf1)
		if err != nil {
			break
		}
		_, err = conn2.Read(recvBuf2)
		if err != nil {
			break
		}
		if (recvBuf1[1]&0x80 == 0x80 && recvBuf2[1]&0x80 == 0x80) && recvBuf1[1]&1 != recvBuf2[1]&1 {
			fmt.Printf("[+] sending %04x <-> %04x\n", recvBuf1, recvBuf2)
			_, err = conn1.Write([]byte{1, recvBuf2[0]})
			if err != nil {
				break
			}
			_, err = conn2.Write([]byte{1, recvBuf1[0]})
			if err != nil {
				break
			}
		} else {
			_, err = conn1.Write([]byte{0, 0})
			if err != nil {
				break
			}
			_, err = conn2.Write([]byte{0, 0})
			if err != nil {
				break
			}
		}
	}
}
