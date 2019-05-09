package server

import (
	"fmt"
	"strings"

	"github.com/cashshuffle/cashshuffle/message"
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
			k := fmt.Sprintf("%s%s", playerPrefix, signed.GetPacket().GetToKey().GetKey())
			msgMap[k] = append(msgMap[k], signed)
		}
	}

	for vk, msgs := range msgMap {
		if vk == broadcastKey {
			pi.broadcastAll(msgs)
		} else {
			sendingPlayer := pi.tracker.playerByConnection(pi.conn)
			if sendingPlayer == nil {
				if debugMode {
					fmt.Printf("[DirectMessage] Ignoring message from %s because player no longer exists\n", getIP(pi.conn))
				}
				return
			}

			player := pi.tracker.playerByVerificationKey(strings.TrimLeft(vk, playerPrefix))
			if player == nil {
				if debugMode {
					fmt.Printf("[DirectMessage] Ignoring message to vk:%s because player no longer exists\n", vk)
				}
				return
			}

			if player == sendingPlayer {
				if debugMode {
					fmt.Printf("[DirectMessage] Ignoring message to self from %s\n", sendingPlayer)
				}
				return
			}

			// stop sending messages after the first error
			if err := writeMessage(player.conn, msgs); err != nil {
				if debugMode {
					fmt.Printf("[DirectMessage] Error writing message to %s: %s\n", player, err)
				}
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
		if debugMode {
			fmt.Printf("[Broadcast] Ignoring message from %s because player no longer exists\n", getIP(pi.conn))
		}
		return
	}

	for _, player := range sender.pool.players {
		// Try to send the message to remaining players even if errors.
		err := writeMessage(player.conn, msgs)
		if (err != nil) && debugMode {
			fmt.Printf("[Broadcast] Continuing to send after write error to %s: %s\n", player, err)
		}
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
		if debugMode {
			fmt.Printf("[ANNOUNCE] Ignoring message from %s because player no longer exists\n", getIP(pi.conn))
		}
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
		err := writeMessage(player.conn, []*message.Signed{&m})
		if (err != nil) && debugMode {
			fmt.Printf("[Broadcast] Continuing to send after write error to %s: %s\n", player, err)
		}
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
