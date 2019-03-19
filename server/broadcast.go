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
func (pi *packetInfo) broadcastMessage() error {
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
			err := pi.broadcastAll(msgs)
			if err != nil {
				// Don't disconnect
				return nil
			}
		} else {
			sendingPlayer := pi.tracker.playerByConnection(pi.conn)
			player := pi.tracker.playerByVerificationKey(strings.TrimLeft(vk, playerPrefix))
			if player == nil {
				// Don't disconnect
				return nil
			}

			if player == sendingPlayer {
				// Don't allow players to send messages to themselves.
				return nil
			}

			err := writeMessage(player.conn, msgs)
			if err != nil {
				// Don't disconnect
				return nil
			}
		}
	}

	return nil
}

// broadcastAll broadcasts to all participants.
func (pi *packetInfo) broadcastAll(msgs []*message.Signed) error {
	pi.tracker.mutex.RLock()
	defer pi.tracker.mutex.RUnlock()

	sender := pi.tracker.connections[pi.conn]

	// If the user has disconnected, then no need to send
	// the broadcast.
	if sender == nil {
		return nil
	}

	for conn, player := range pi.tracker.connections {
		if (sender.pool != player.pool) || sender.pool.IsBanned(player) {
			continue
		}

		err := writeMessage(conn, msgs)
		if err != nil {
			return err
		}
	}

	return nil
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

	for conn, player := range pi.tracker.connections {
		if sender.pool != player.pool || sender.pool.IsBanned(player) {
			continue
		}

		m := message.Signed{
			Packet: &message.Packet{
				Phase:  message.Phase_ANNOUNCEMENT,
				Number: uint32(pi.tracker.poolSize),
			},
		}

		err := writeMessage(conn, []*message.Signed{&m})
		if err != nil {
			// Don't disconnect
			return
		}

		// the player now has an obligation to send verification key or
		// receive ban effects
		player.isPassive = true
	}
}

func (pi *packetInfo) broadcastJoinedPool(p *playerData) error {
	m := message.Signed{
		Packet: &message.Packet{
			Number: p.number,
		},
	}

	return pi.broadcastAll([]*message.Signed{&m})
}
