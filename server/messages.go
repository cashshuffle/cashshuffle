package server

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/cashshuffle/cashshuffle/message"
	"github.com/golang/protobuf/proto"
)

const (
	maxMessageLength = 64 * 1024
)

var (
	// magicBytes are the bytes starting each message
	magicBytes = []byte{66, 188, 195, 38, 105, 70, 120, 115}

	// headerLength is the length of the magic string
	// and the message length.
	headerLength = 12
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

		// If a malicious client is connecting and disconnecting
		// quickly it is possible that playerData will be nil.
		// No need to return an error, just don't register them.
		if playerData == nil {
			return nil
		}

		if pi.tracker.getPoolSize(playerData.pool) == pi.tracker.poolSize {
			pi.announceStart()
		} else {
			pi.broadcastJoinedPool(playerData)
		}

		return nil
	}

	if err := pi.verifyMessage(); err != nil {
		return err
	}

	if err := pi.checkBanMessage(); err != nil {
		if err.Error() == playerBannedErrorMessage {
			return nil
		}
		return err
	}

	return pi.broadcastMessage()
}

// processMessages reads messages from the connection and begins processing.
func processMessages(conn net.Conn, c chan *packetInfo, t *Tracker) {
	defer t.remove(conn)

	scanner := bufio.NewScanner(conn)
	scanner.Split(bufio.ScanBytes)

	var validMagic bool
	var numReadBytes int
	needFrame := true

	var b bytes.Buffer
	var mb bytes.Buffer

	for {
		for scanner.Scan() {
			scanBytes := scanner.Bytes()
			b.Write(scanBytes)

			if needFrame {
				if b.Len() >= headerLength {
					validMagic, numReadBytes = processFrame(&b)

					if !validMagic {
						fmt.Fprintf(os.Stderr, "[Error] %s\n", "invalid magic")
						return
					}

					if numReadBytes <= 0 || numReadBytes > maxMessageLength {
						fmt.Fprintf(os.Stderr, "[Error] %s\n", "invalid message length")
						return
					}

					needFrame = false
				}

				continue
			}

			if b.Len() >= numReadBytes {
				msg := make([]byte, numReadBytes)
				b.Read(msg)

				mb.Write(msg)

				needFrame = true
				break
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "[Error] %s\n", err.Error())
			break
		}

		if mb.Len() == 0 {
			break
		}

		// Extend the deadline, we got a valid full message.
		conn.SetDeadline(time.Now().Add(deadline))

		if err := sendToPacketInfoChan(&mb, conn, c, t); err != nil {
			fmt.Fprintf(os.Stderr, "[Error] %s\n", err.Error())
			break
		}
	}
}

// processFrame takes a buffer and returns whether
// the magic bytes are correct, and the length of
// the expected message.
func processFrame(b *bytes.Buffer) (bool, int) {
	magic := make([]byte, len(magicBytes))
	b.Read(magic)

	lenBytes := make([]byte, 4)
	b.Read(lenBytes)

	return bytes.Equal(magic, magicBytes), int(binary.BigEndian.Uint32(lenBytes))
}

// sendToPacketInfoChan takes a byte buffer containing a protobuf message,
// unmarshals it, creates a packetInfo, then sends it over the packetInfo
// channel.
func sendToPacketInfoChan(b *bytes.Buffer, conn net.Conn, c chan *packetInfo, t *Tracker) error {
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
