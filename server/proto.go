package server

import (
	"encoding/binary"
	"net"
	"time"

	"github.com/cashshuffle/cashshuffle/message"

	"github.com/golang/protobuf/proto"

	log "github.com/sirupsen/logrus"
)

const (
	deadline        = 180 * time.Second
	connectDeadline = 15 * time.Second
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

	_, err = conn.Write(frameMessage(reply))
	if err != nil {
		return err
	}

	log.Debugf(logCommunication+"Sent by %s: %s\n", getIP(conn), packets)

	// Extend the deadline, we just sent a message.
	if err = conn.SetDeadline(time.Now().Add(deadline)); err != nil {
		// Failing to set the deadline could be due to the client getting
		// ignored due to some bad behavior. Do not consider the write itself
		// a failure due to failure to set the deadline. The client will drop
		// off eventually after connection is broken anyway.
		log.Debugf(logCommunication+"Error setting deadline after successful write: %s\n", err)
	}

	return nil
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
