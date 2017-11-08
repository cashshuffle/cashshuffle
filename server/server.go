package server

import (
	"fmt"
	"net"
)

var debugMode bool

// Start brings up the TCP server.
func Start(port int, cert string, key string, poolSize int, debug bool) (err error) {
	var listener net.Listener

	debugMode = debug

	if tlsEnabled(cert, key) {
		listener, err = createTLSListener(port, cert, key)
		if err != nil {
			return err
		}
	} else {
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return err
		}
	}

	defer listener.Close()

	t := &tracker{
		poolSize: poolSize,
	}
	t.init()

	packetInfoChan := make(chan *packetInfo)
	go startPacketInfoChan(packetInfoChan)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleConnection(conn, packetInfoChan, t)
	}
}

func handleConnection(conn net.Conn, c chan *packetInfo, tracker *tracker) {
	defer conn.Close()
	defer tracker.remove(conn)

	processMessages(conn, c, tracker)

	return
}
