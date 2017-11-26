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
		to := signed.GetPacket().GetTo()

		if to == nil {
			msgMap[broadcastKey] = append(msgMap[broadcastKey], signed)
		} else {
			k := fmt.Sprintf("%s%s", playerPrefix, signed.GetPacket().GetTo().String())
			msgMap[k] = append(msgMap[k], signed)
		}
	}

	for player, msgs := range msgMap {
		if player == broadcastKey {
			err := pi.broadcastAll(msgs)
			if err != nil {
				pi.broadcastNewRound()

				// Don't disconnect, we broadcasted a new round.
				return nil
			}
		} else {
			td := pi.tracker.getVerificationKeyData(strings.TrimLeft(player, playerPrefix))
			if td == nil {
				pi.broadcastNewRound()

				// Don't disconnect
				return nil
			}

			err := writeMessage(td.conn, msgs)
			if err != nil {
				pi.broadcastNewRound()

				// Don't disconnect
				return nil
			}
		}
	}

	return nil
}

// broadcastAll broadcasts to all participants.
func (pi *packetInfo) broadcastAll(msgs []*message.Signed) error {
	pi.tracker.mutex.Lock()
	defer pi.tracker.mutex.Unlock()

	playerData := pi.tracker.connections[pi.conn]

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
func (pi *packetInfo) broadcastNewRound() {
	pi.tracker.mutex.Lock()
	defer pi.tracker.mutex.Unlock()

	playerData := pi.tracker.connections[pi.conn]

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

	return
}

// announceStart sends an annoucement message if the pool
// is full.
func (pi *packetInfo) announceStart() {
	pi.tracker.mutex.Lock()
	defer pi.tracker.mutex.Unlock()

	playerData := pi.tracker.connections[pi.conn]

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
			pi.broadcastNewRound()

			// Don't disconnect
			return
		}
	}

	return
}
