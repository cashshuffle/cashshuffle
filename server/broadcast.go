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
			td := pi.tracker.getVerificationKeyData(strings.TrimLeft(player, playerPrefix))
			if td == nil {
				pi.broadcastNewRound(true)

				// Don't disconnect
				return nil
			}

			err := writeMessage(td.conn, msgs)
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

	playerData := pi.tracker.connections[pi.conn]

	// If the user has disconnected, then no need to send
	// the broadcast.
	if playerData == nil {
		return nil
	}

	for conn, td := range pi.tracker.connections {
		if (playerData.pool != td.pool) || pi.tracker.banned(td) {
			continue
		}

		err := writeMessage(conn, msgs)
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

	playerData := pi.tracker.connections[pi.conn]

	// If the user has disconnected, then no need to send
	// the broadcast.
	if playerData == nil {
		return
	}

	for conn, td := range pi.tracker.connections {
		if playerData.pool != td.pool || pi.tracker.banned(td) {
			continue
		}

		m := message.Signed{
			Packet: &message.Packet{
				Session: td.sessionID,
				Number:  td.number,
				Message: &message.Message{
					Str: "New round",
				},
			},
		}

		err := writeMessage(conn, []*message.Signed{&m})
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

	playerData := pi.tracker.connections[pi.conn]

	// If the user has disconnected, then no need to send
	// the broadcast.
	if playerData == nil {
		return
	}

	for conn, td := range pi.tracker.connections {
		if playerData.pool != td.pool || pi.tracker.banned(td) {
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
			pi.broadcastNewRound(false)

			// Don't disconnect
			return
		}
	}
}

func (pi *packetInfo) broadcastJoinedPool(td *trackerData) error {
	m := message.Signed{
		Packet: &message.Packet{
			Number: td.number,
		},
	}

	return pi.broadcastAll([]*message.Signed{&m})
}
