package server

import (
	"net"

	"github.com/cashshuffle/cashshuffle/message"
	"github.com/golang/protobuf/proto"
)

// writeMessage writes a *message.Signed to the connection via protobuf.
func writeMessage(conn net.Conn, m *message.Signed) error {
	reply, err := proto.Marshal(m)
	if err != nil {
		return err
	}

	_, err = conn.Write(reply)
	if err != nil {
		return err
	}

	_, err = conn.Write(breakBytes)
	if err != nil {
		return err
	}

	return nil
}
