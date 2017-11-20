package server

import (
	"fmt"
	"net"
	"time"

	"github.com/cashshuffle/cashshuffle/message"
	"github.com/golang/protobuf/proto"
)

// writeMessage writes a *message.Signed to the connection via protobuf.
func writeMessage(conn net.Conn, msgs []*message.Signed) error {
	packets := &message.Packets{
		Packet: msgs,
	}

	reply, err := proto.Marshal(packets)
	if err != nil {
		return err
	}

	if debugMode {
		fmt.Println("[Sent]", packets)
	}

	time.Sleep(250 * time.Millisecond)

	_, err = conn.Write(append(reply, breakBytes...))
	if err != nil {
		return err
	}

	return nil
}
