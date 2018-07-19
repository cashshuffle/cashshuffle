package server

import (
	"net"

	"github.com/cashshuffle/cashshuffle/message"
)

// packetInfo is a type to represent the received Packets message
// and the current connection.
type packetInfo struct {
	message *message.Packets
	conn    net.Conn
	tracker *Tracker
}
