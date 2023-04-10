package gbc

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"time"
)

type SerialPort struct {
	GBC   *Console
	ticks int
	freq  int

	receiveChan chan uint8
	sendChan    chan uint8

	serverOK bool
	enabled  bool
	SB, SC   uint8
}

const (
	HOST = "localhost"
	PORT = "3333"
	TYPE = "tcp"
)

func (sp *SerialPort) Save(encoder *gob.Encoder) {
	panicIfErr(encoder.Encode(sp.SB))
	panicIfErr(encoder.Encode(sp.SC))
	panicIfErr(encoder.Encode(sp.enabled))
	panicIfErr(encoder.Encode(sp.ticks))
	panicIfErr(encoder.Encode(sp.freq))
}

func (sp *SerialPort) Load(decoder *gob.Decoder) {
	panicIfErr(decoder.Decode(&sp.SB))
	panicIfErr(decoder.Decode(&sp.SC))
	panicIfErr(decoder.Decode(&sp.enabled))
	panicIfErr(decoder.Decode(&sp.ticks))
	panicIfErr(decoder.Decode(&sp.freq))
}

func handleRead(sp *SerialPort, conn *net.TCPConn) {
	var buf = make([]byte, 1)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			break
		}
		sp.receiveChan <- buf[0]
	}
}

func handleWrite(sp *SerialPort, conn *net.TCPConn) {
	var buf = make([]byte, 1)
	for {
		buf[0] = <-sp.sendChan
		_, err := conn.Write(buf)
		if err != nil {
			break
		}
	}
}

func serverLoop(sp *SerialPort) {
	for {
		tcpAddr, _ := net.ResolveTCPAddr(TYPE, HOST+":"+PORT)
		conn, err := net.DialTCP(TYPE, nil, tcpAddr)
		if err != nil {
			log.Printf("unable to dial serial server, sleeping")
			time.Sleep(time.Second * 10)
			continue
		}

		sp.serverOK = true
		go handleRead(sp, conn)
		handleWrite(sp, conn)
		conn.Close()
		sp.serverOK = false
	}
}

func MakeSerialPort(cons *Console) *SerialPort {
	sp := &SerialPort{
		GBC:         cons,
		freq:        8192,
		sendChan:    make(chan uint8),
		receiveChan: make(chan uint8),
	}
	go serverLoop(sp)
	return sp
}

func (sp *SerialPort) WriteSB(value uint8) {
	sp.SB = value
	if sp.enabled && sp.serverOK {
		sp.sendChan <- value
	}
}

func (sp *SerialPort) WriteSC(value uint8) {
	sp.SC = value

	wasEnabled := sp.enabled
	sp.enabled = value&0x80 != 0 && sp.serverOK
	if !wasEnabled && sp.enabled {
		sp.sendChan <- sp.SB
	}
}

func (sp *SerialPort) Tick(ticks int) {
	if sp.enabled && sp.serverOK {
		sp.ticks += ticks * 4
		if sp.ticks >= sp.freq {
			sp.ticks -= sp.freq

			select {
			case v, ok := <-sp.receiveChan:
				if ok {
					sp.SB = v
				} else {
					sp.SB = 0xFF
				}
			default:
				fmt.Println("No value ready, moving on.")
			}
			if sp.SC&1 == 0 {
				sp.GBC.CPU.SetInterrupt(InterruptSerial.Mask)
			}
		}
	} else {
		sp.enabled = false
		sp.ticks = 0
	}
}
