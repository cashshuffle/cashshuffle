package server

import (
	"fmt"
	"net"
)

// Start brings up the TCP server.
func Start(port int, cert string, key string) (err error) {
	var listener net.Listener

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

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	return
}
