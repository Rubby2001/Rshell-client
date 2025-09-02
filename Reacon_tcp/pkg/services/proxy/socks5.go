package Proxy

import (
	"Reacon/pkg/services/proxy/bufferpool"
	"Reacon/pkg/services/proxy/mux"
	"crypto/tls"
	"log"
	"math"
	"net"
)

var (
	bufferPool  = bufferpool.NewPool(math.MaxUint16)
	magicPacket = [64]byte{
		0x6a, 0x1d, 0x3e, 0x74, 0x8b, 0x99, 0x5a, 0x7f,
		0xca, 0xef, 0x33, 0x88, 0xac, 0x44, 0x52, 0xbd,
		0x1e, 0x5f, 0x39, 0xd4, 0x6c, 0xb3, 0x72, 0xf8,
		0x21, 0x9d, 0x54, 0x68, 0x91, 0xab, 0x43, 0xee,
		0x4c, 0x7a, 0x90, 0x26, 0xf7, 0x35, 0x9b, 0x5e,
		0xd1, 0x88, 0x4f, 0xbc, 0x2a, 0x67, 0x91, 0xe4,
		0xaf, 0x34, 0xcd, 0x89, 0x60, 0x18, 0xa2, 0xde,
		0x77, 0x93, 0xfb, 0x02, 0x6e, 0x11, 0xc3, 0xf0,
	}
)
var Session *mux.Mux

// Start a socks5 server and tunnel the traffic to the server at address.
func ReverseSocksAgent(serverAddress, psk string, useTLS bool) {
	log.Println("Connecting to socks server at " + serverAddress)

	var conn net.Conn
	var err error

	if useTLS {
		conn, err = tls.Dial("tcp", serverAddress, nil)
	} else {
		conn, err = net.Dial("tcp", serverAddress)
	}

	if err != nil {
		log.Fatalln(err.Error())
	}

	//1. 先发送magicPacket

	_, err = conn.Write(magicPacket[:])
	if err != nil {
		log.Fatalln(err.Error())
	}

	log.Println("Connected")

	// 多路复用的一个mux
	Session = mux.Server(conn, psk)

	for {
		stream, err := Session.AcceptStream()
		if err != nil {
			log.Println(err.Error())
			break
		}
		go func() {
			// Note ServeConn() will take overship of stream and close it.
			if err := ServeConn(stream); err != nil && err != mux.ErrPeerClosedStream {
				log.Println(err.Error())
			}
		}()
	}

	Session.Close()
}
