package server

import (
	"errors"

	"github.com/cashshuffle/cashshuffle/message"
)

// broadcastMessage processes messages and either broadcasts
// them to all connected users, or to a single user.
func (sc *signedConn) broadcastMessage() error {
	to := sc.message.GetPacket().GetTo()

	if to == nil {
		err := sc.broadcastAll()
		if err != nil {
			sc.broadcastNewRound()

			// Don't disconnect, we broadcasted a new round.
			return nil
		}
	} else {
		td := sc.tracker.getVerificationKeyData(to.String())
		if td == nil {
			return errors.New("peer disconnected")
		}

		err := writeMessage(td.conn, sc.message)
		if err != nil {
			sc.broadcastNewRound()

			// Don't disconnect
			return nil
		}
	}

	return nil
}

// broadcastAll broadcasts to all participants.
func (sc *signedConn) broadcastAll() error {
	sc.tracker.mutex.Lock()
	defer sc.tracker.mutex.Unlock()

	playerData := sc.tracker.connections[sc.conn]

	for conn, td := range sc.tracker.connections {
		if playerData.pool != td.pool {
			continue
		}

		err := writeMessage(conn, sc.message)
		if err != nil {
			return err
		}
	}

	return nil
}

// broadcastNewRound broadcasts a new round.
func (sc *signedConn) broadcastNewRound() {
	sc.tracker.mutex.Lock()
	defer sc.tracker.mutex.Unlock()

	playerData := sc.tracker.connections[sc.conn]

	for conn, td := range sc.tracker.connections {
		if playerData.pool != td.pool {
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

		err := writeMessage(conn, &m)
		if err != nil {
			continue
		}
	}

	return
}

// announceStart sends an annoucement message if the pool
// is full.
func (sc *signedConn) announceStart() {
	sc.tracker.mutex.Lock()
	defer sc.tracker.mutex.Unlock()

	playerData := sc.tracker.connections[sc.conn]

	for conn, td := range sc.tracker.connections {
		if playerData.pool != td.pool {
			continue
		}

		m := message.Signed{
			Packet: &message.Packet{
				Phase:  message.Phase_ANNOUNCEMENT,
				Number: uint32(sc.tracker.poolSize),
			},
		}

		err := writeMessage(conn, &m)
		if err != nil {
			sc.broadcastNewRound()

			// Don't disconnect
			return
		}
	}

	return
}
