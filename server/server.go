package server

import (
	"fmt"
	"net"

	"golang.org/x/crypto/acme/autocert"
)

var debugMode bool

// Start brings up the TCP server.
func Start(ip string, port int, cert string, key string, debug bool, t *Tracker, m *autocert.Manager) (err error) {
	var listener net.Listener

	debugMode = debug

	if tlsEnabled(cert, key, m) {
		listener, err = createTLSListener(ip, port, cert, key, m)
		if err != nil {
			return err
		}
	} else {
		listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port))
		if err != nil {
			return err
		}
	}

	defer listener.Close()

	packetInfoChan := make(chan *packetInfo)
	go startPacketInfoChan(packetInfoChan)

	fmt.Printf("Shuffle Listening on TCP %s:%d (pool size: %d)\n", ip, port, t.poolSize)
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleConnection(conn, packetInfoChan, t)
	}
}

func handleConnection(conn net.Conn, c chan *packetInfo, tracker *Tracker) {
	defer conn.Close()
	defer tracker.remove(conn)

	processMessages(conn, c, tracker)
}
