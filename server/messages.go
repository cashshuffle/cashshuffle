package server

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"

	"github.com/cashshuffle/cashshuffle/message"
	"github.com/golang/protobuf/proto"
)

const (
	maxMessageLength = 64 * 1024
)

var (
	// breakBytes are the bytes that delimit each protobuf message
	// This represents the character ⏎
	breakBytes = []byte{226, 143, 142}
)

// startPacketInfoChan starts a loop reading messages.
func startPacketInfoChan(c chan *packetInfo) {
	for {
		pi := <-c
		err := pi.processReceivedMessage()
		if err != nil {
			pi.conn.Close()
			fmt.Fprintf(os.Stderr, "[Error] %s\n", err.Error())
		}
	}
}

// processReceivedMessage reads the message and processes it.
func (pi *packetInfo) processReceivedMessage() error {
	// If we are not tracking the connection yet, the user must be
	// registering with the server.
	if pi.tracker.getTrackerData(pi.conn) == nil {
		err := pi.registerClient()
		if err != nil {
			return err
		}

		playerData := pi.tracker.getTrackerData(pi.conn)

		if pi.tracker.getPoolSize(playerData.pool) == pi.tracker.poolSize {
			pi.announceStart()
		}

		return nil
	}

	if err := pi.verifyMessage(); err != nil {
		return err
	}

	err := pi.broadcastMessage()
	return err
}

// processMessages reads messages from the connection and begins processing.
func processMessages(conn net.Conn, c chan *packetInfo, t *tracker) {
	scanner := bufio.NewScanner(conn)
	scanner.Split(bufio.ScanBytes)

	for {
		var b bytes.Buffer

		for scanner.Scan() {
			scanBytes := scanner.Bytes()

			if len(b.String()) > maxMessageLength {
				fmt.Fprintln(os.Stderr, "[Error] message too long")
				return
			}

			b.Write(scanBytes)

			if breakScan(b) {
				b.Truncate(b.Len() - 3)
				break
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "[Error] %s\n", err.Error())
			break
		}

		if b.Len() == 0 {
			continue
		}

		if err := sendToPacketInfoChan(&b, conn, c, t); err != nil {
			fmt.Fprintf(os.Stderr, "[Error] %s\n", err.Error())
			break
		}
	}
}

// sendToPacketInfoChan takes a byte buffer containing a protobuf message,
// unmarshals it, creates a packetInfo, then sends it over the packetInfo
// channel.
func sendToPacketInfoChan(b *bytes.Buffer, conn net.Conn, c chan *packetInfo, t *tracker) error {
	defer b.Reset()

	pdata := new(message.Packets)

	err := proto.Unmarshal(b.Bytes(), pdata)
	if err != nil {
		if debugMode {
			fmt.Println("[Error] Unmarshal failed:", b.Bytes())
		}
		return err
	}

	if debugMode {
		fmt.Println("[Received]", pdata)
	}

	data := &packetInfo{
		message: pdata,
		conn:    conn,
		tracker: t,
	}

	c <- data

	return nil
}

// breakScan checks if a byte sequence is the break point on the scanner.
func breakScan(buf bytes.Buffer) bool {
	len := buf.Len()

	if len > 3 {
		payload := buf.Bytes()
		bs := []byte{
			payload[len-3],
			payload[len-2],
			payload[len-1],
		}

		for i := range bs {
			if bs[i] != breakBytes[i] {
				return false
			}
		}

		return true
	}

	return false
}
