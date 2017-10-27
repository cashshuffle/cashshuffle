package server

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/cashshuffle/cashshuffle/message"
	"github.com/golang/protobuf/proto"
)

// startSignedChan starts a loop reading messages.
func startSignedChan(c chan *message.Signed) {
	for {
		message := <-c
		processReceivedData(message)
	}
}

// processReceivedData reads the message and processes it.
func processReceivedData(data *message.Signed) {
	p := data.GetPacket()

	// Data processing goes here. Right now we just print each message.
	fmt.Println(p)
}

// processMessages reads messages from the connection and converts
// them message.Signed for processing.
func processMessages(conn net.Conn, c chan *message.Signed) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Error] %s", err.Error())
		return
	}

	pdata := new(message.Signed)
	err = proto.Unmarshal(buf.Bytes(), pdata)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Error] %s", err.Error())
		return
	}

	c <- pdata
}
