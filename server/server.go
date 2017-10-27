package server

import (
	"fmt"
	"net"

	"github.com/cashshuffle/cashshuffle/message"
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

	t := &tracker{}
	t.init()

	signedChan := make(chan *message.Signed)
	go startSignedChan(signedChan)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleConnection(conn, signedChan, t)
	}
}

func handleConnection(conn net.Conn, c chan *message.Signed, tracker *tracker) {
	defer conn.Close()
	defer tracker.remove(&conn)

	data := &trackerData{}
	tracker.add(&conn, data)

	processMessages(conn, c)

	return
}
