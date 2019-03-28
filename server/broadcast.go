package server

import (
	"fmt"
	"strings"

	"github.com/cculianu/cashshuffle/message"
)

var (
	playerPrefix = "Player"
	broadcastKey = "Broadcast"
)

// broadcastMessage processes messages and either broadcasts
// them to all connected users, or to a single user.
func (pi *packetInfo) broadcastMessage() {
	msgMap := make(map[string][]*message.Signed)

	for _, signed := range pi.message.GetPacket() {
		to := signed.GetPacket().GetToKey()

		if to == nil {
			msgMap[broadcastKey] = append(msgMap[broadcastKey], signed)
		} else {
			k := fmt.Sprintf("%s%s", playerPrefix, signed.GetPacket().GetToKey().String())
			msgMap[k] = append(msgMap[k], signed)
		}
	}

	for vk, msgs := range msgMap {
		if vk == broadcastKey {
			pi.broadcastAll(msgs)
		} else {
			sendingPlayer := pi.tracker.playerByConnection(pi.conn)
			player := pi.tracker.playerByVerificationKey(strings.TrimLeft(vk, playerPrefix))
			if player == nil {
				return
			}

			if player == sendingPlayer {
				// Don't allow players to send messages to themselves.
				return
			}

			// stop sending messages after the first error
			if err := writeMessage(player.conn, msgs); err != nil {
				return
			}
		}
	}
}

// broadcastAll broadcasts to all participants.
func (pi *packetInfo) broadcastAll(msgs []*message.Signed) {
	pi.tracker.mutex.RLock()
	defer pi.tracker.mutex.RUnlock()

	sender := pi.tracker.connections[pi.conn]

	// If the user has disconnected, then no need to send
	// the broadcast.
	if sender == nil {
		return
	}

	for _, player := range sender.pool.players {
		// Try to send the message to remaining players even if errors.
		_ = writeMessage(player.conn, msgs)
	}
}

// announceStart sends an announcement message if the pool
// is full.
func (pi *packetInfo) announceStart() {
	pi.tracker.mutex.RLock()
	defer pi.tracker.mutex.RUnlock()

	sender := pi.tracker.connections[pi.conn]

	// If the user has disconnected, then no need to send
	// the broadcast.
	if sender == nil {
		return
	}

	for _, player := range sender.pool.players {
		m := message.Signed{
			Packet: &message.Packet{
				Phase:  message.Phase_ANNOUNCEMENT,
				Number: uint32(pi.tracker.poolSize),
			},
		}
		// The player now has an obligation to send verification key.
		// Since we cannot differentiate between a user ignoring the message
		// and an honest miss, we assume the user always receives the message.
		player.isPassive = true

		// Try to send the message to remaining players even if errors.
		_ = writeMessage(player.conn, []*message.Signed{&m})
	}
}

func (pi *packetInfo) broadcastJoinedPool(p *PlayerData) {
	m := message.Signed{
		Packet: &message.Packet{
			Number: p.number,
		},
	}

	pi.broadcastAll([]*message.Signed{&m})
}
