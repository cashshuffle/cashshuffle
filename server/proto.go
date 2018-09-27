package server

import (
	"encoding/binary"
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

	_, err = conn.Write(frameMessage(reply))
	return err
}

// frameMessage sets up the message to be sent
// over the wire. This is sent as
// [magicBytes][length][message]
func frameMessage(reply []byte) []byte {
	msg := []byte{}

	msg = append(msg, magicBytes...)

	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, uint32(len(reply)))
	msg = append(msg, bs...)

	msg = append(msg, reply...)

	return msg
}
