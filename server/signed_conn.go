package server

import (
	"net"

	"github.com/cashshuffle/cashshuffle/message"
)

// signedConn is a type to represent the signed message
// and the current connection.
type signedConn struct {
	message *message.Signed
	conn    net.Conn
	tracker *tracker
	pool    int
}
