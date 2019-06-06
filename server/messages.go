package server

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net"
	"time"

	"github.com/cashshuffle/cashshuffle/message"

	"github.com/golang/protobuf/proto"

	log "github.com/sirupsen/logrus"
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
			log.Warnf(logCommunication+"Message processor error: %s\n", err)
		}
	}
}

// processReceivedMessage reads the message and processes it.
func (pi *packetInfo) processReceivedMessage() error {
	var player *PlayerData
	// If we are not tracking the connection yet, the user must be
	// registering with the server.
	if pi.tracker.playerByConnection(pi.conn) == nil {
		err := pi.registerClient()
		if err != nil {
			return err
		}

		player = pi.tracker.playerByConnection(pi.conn)

		// If a malicious client is connecting and disconnecting
		// quickly it is possible that PlayerData will be nil.
		// No need to return an error, just don't register them.
		if player == nil {
			return nil
		}

		if player.pool.IsFrozen() {
			pi.announceStart()
		} else {
			pi.broadcastJoinedPool(player)
		}

		return nil
	}

	if err := pi.verifyMessage(); err != nil {
		return err
	}

	// At this point we are confident that the user has at least attempted
	// to broadcast a valid message with their verification key. We unset
	// the passive flag so that they will not get a ban score / p2p ban
	// when leaving the pool.
	// At least this must happen before blame checking logic happens.
	if player = pi.tracker.playerByConnection(pi.conn); player != nil {
		player.isPassive = false
	}

	if err := pi.checkBlameMessage(); err != nil {
		return err
	}

	pi.broadcastMessage()
	return nil
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
						log.Warn(logCommunication + "Invalid magic\n")
						return
					}

					if numReadBytes <= 0 || numReadBytes > maxMessageLength {
						log.Warnf(logCommunication+"Invalid message length: %d\n", numReadBytes)
						return
					}

					needFrame = false
				}

				continue
			}

			if b.Len() >= numReadBytes {
				msg := make([]byte, numReadBytes)
				if _, err := b.Read(msg); err != nil {
					log.Warnf(logCommunication+"Error reading from message buffer: %s\n", err)
					return
				}

				mb.Write(msg)

				needFrame = true
				break
			}
		}

		if err := scanner.Err(); err != nil {
			log.Warnf(logCommunication+"Error scanning message: %s\n", err)
			return
		}

		if mb.Len() == 0 {
			log.Warn(logCommunication + "0-length message\n")
			return
		}

		// Extend the deadline, we got a valid full message.
		if err := conn.SetDeadline(time.Now().Add(deadline)); err != nil {
			// Failing to set the deadline could be due to the client getting
			// disconnected for some reason. Do not consider the read itself
			// a failure due to failure to set the deadline. The client will drop
			// off eventually after connection is broken anyway.
			log.Warnf(logCommunication+"Error setting deadline after successful receive: %s\n", err)
		}

		if err := sendToPacketInfoChan(&mb, conn, c, t); err != nil {
			log.Warnf(logCommunication+"Error sending packet: %s\n", err)
			return
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
		log.Debugf(logCommunication+"Unmarshal failed: %v\n", b.Bytes())
		return err
	}

	log.Debugf(logCommunication+"Received from %s: %s\n", getIP(conn), pdata)

	data := &packetInfo{
		message: pdata,
		conn:    conn,
		tracker: t,
	}

	c <- data

	return nil
}
