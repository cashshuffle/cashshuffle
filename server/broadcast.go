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

	for player, msgs := range msgMap {
		if player == broadcastKey {
			err := pi.broadcastAll(msgs)
			if err != nil {
				pi.broadcastNewRound(true)

				// Don't disconnect, we broadcasted a new round.
				return nil
			}
		} else {
			player := pi.tracker.getVerificationKeyPlayer(strings.TrimLeft(player, playerPrefix))
			if player == nil {
				pi.broadcastNewRound(true)

				// Don't disconnect
				return nil
			}

			err := writeMessage(player.conn, msgs)
			if err != nil {
				pi.broadcastNewRound(true)

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

	player := pi.tracker.connections[pi.conn]

	// If the user has disconnected, then no need to send
	// the broadcast.
	if player == nil {
		return nil
	}

	for otherConn, other := range pi.tracker.connections {
		if (player.pool != other.pool) || pi.tracker.bannedByPool(other) {
			continue
		}

		err := writeMessage(otherConn, msgs)
		if err != nil {
			return err
		}
	}

	return nil
}

// broadcastNewRound broadcasts a new round.
func (pi *packetInfo) broadcastNewRound(lock bool) {
	if lock {
		pi.tracker.mutex.RLock()
		defer pi.tracker.mutex.RUnlock()
	}

	player := pi.tracker.connections[pi.conn]

	// If the user has disconnected, then no need to send
	// the broadcast.
	if player == nil {
		return
	}

	for otherConn, other := range pi.tracker.connections {
		if player.pool != other.pool || pi.tracker.bannedByPool(other) {
			continue
		}

		m := message.Signed{
			Packet: &message.Packet{
				Session: other.sessionID,
				Number:  other.number,
				Message: &message.Message{
					Str: "New round",
				},
			},
		}

		err := writeMessage(otherConn, []*message.Signed{&m})
		if err != nil {
			continue
		}
	}
}

// announceStart sends an annoucement message if the pool
// is full.
func (pi *packetInfo) announceStart() {
	pi.tracker.mutex.RLock()
	defer pi.tracker.mutex.RUnlock()

	player := pi.tracker.connections[pi.conn]

	// If the user has disconnected, then no need to send
	// the broadcast.
	if player == nil {
		return
	}

	for otherConn, other := range pi.tracker.connections {
		if player.pool != other.pool || pi.tracker.bannedByPool(other) {
			continue
		}

		m := message.Signed{
			Packet: &message.Packet{
				Phase:  message.Phase_ANNOUNCEMENT,
				Number: uint32(pi.tracker.poolSize),
			},
		}

		err := writeMessage(otherConn, []*message.Signed{&m})
		if err != nil {
			pi.broadcastNewRound(false)

			// Don't disconnect
			return
		}
	}
}

func (pi *packetInfo) broadcastJoinedPool(player *playerData) error {
	m := message.Signed{
		Packet: &message.Packet{
			Number: player.number,
		},
	}

	return pi.broadcastAll([]*message.Signed{&m})
}
