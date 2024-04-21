package gbc

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
)

var SERIAL_TICK_COUNT = 4096

type Serial struct {
	GBC           *Console
	SB            uint8
	SC            uint8
	serialCounter int

	host               string
	port               int
	conn               *net.TCPConn
	txSB, txSC, rxSB   chan uint8
	receivedData, quit chan bool
}

func (s *Serial) Save(encoder *gob.Encoder) {
	panicIfErr(encoder.Encode(s.serialCounter))
	panicIfErr(encoder.Encode(s.SB))
	panicIfErr(encoder.Encode(s.SC))
}

func (s *Serial) Load(decoder *gob.Decoder) {
	panicIfErr(decoder.Decode(&s.serialCounter))
	panicIfErr(decoder.Decode(&s.SB))
	panicIfErr(decoder.Decode(&s.SC))
}

func connThread(s *Serial) {
	recvBuffer := make([]byte, 2)
	for !<-s.quit {
		sb := <-s.txSB
		sc := <-s.txSC
		_, err := s.conn.Write([]byte{sb, sc})
		if err != nil {
			break
		}

		_, err = s.conn.Read(recvBuffer)
		if err != nil {
			break
		}
		if recvBuffer[0] == 0 {
			s.receivedData <- false
		} else {
			s.receivedData <- true
			s.rxSB <- recvBuffer[1]
		}
	}

	s.conn.Close()
	s.conn = nil
}

func (s *Serial) connectToRemote() bool {
	if s.conn != nil {
		return false
	}
	tcpServer, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", s.host, s.port))
	if err != nil {
		s.conn = nil
		return false
	}
	conn, err := net.DialTCP("tcp", nil, tcpServer)
	if err != nil {
		s.conn = nil
		return false
	}

	s.conn = conn
	s.txSB = make(chan uint8)
	s.txSC = make(chan uint8)
	s.rxSB = make(chan uint8)
	s.receivedData = make(chan bool)
	s.quit = make(chan bool)

	go connThread(s)
	return true
}

func MakeSerial(c *Console, host string, port int) *Serial {
	s := &Serial{
		GBC:           c,
		host:          host,
		port:          port,
		serialCounter: 0,
	}

	if s.connectToRemote() {
		log.Println("connected to remote")
	}
	return s
}

func (s *Serial) isConnected() bool {
	return s.conn != nil
}

func (s *Serial) Tick(ticks int) {
	s.serialCounter += ticks * 4
	if s.serialCounter >= SERIAL_TICK_COUNT {
		s.serialCounter -= SERIAL_TICK_COUNT

		if s.isConnected() {
			s.quit <- false
			s.txSB <- s.SB
			s.txSC <- s.SC
		}

		shouldTriggerInterrupt := false
		if s.isConnected() {
			// blocking!
			if <-s.receivedData {
				s.SB = <-s.rxSB
				shouldTriggerInterrupt = true
			}
		} else if s.SC&0x81 == 0x81 {
			shouldTriggerInterrupt = true
		}
		if shouldTriggerInterrupt {
			s.SC &= 1
			s.GBC.CPU.SetInterrupt(InterruptSerial.Mask)
		}
	}
}

func (s *Serial) Delete() {
	if s.isConnected() {
		s.quit <- true
	}
}
