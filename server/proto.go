package server

import (
	"fmt"
	"net"

	"github.com/cashshuffle/cashshuffle/message"
	"github.com/golang/protobuf/proto"
)

// writeMessage writes a *message.Signed to the connection via protobuf.
func writeMessage(conn net.Conn, m *message.Signed) error {
	packets := &message.Packets{
		Packet: []*message.Signed{m},
	}

	reply, err := proto.Marshal(packets)
	if err != nil {
		return err
	}

	if debugMode {
		fmt.Println("[Sent]", packets)
	}

	_, err = conn.Write([]byte(fmt.Sprintf("%s%s", reply, breakBytes)))
	if err != nil {
		return err
	}

	return nil
}
