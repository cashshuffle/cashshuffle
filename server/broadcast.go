package server

import (
	"errors"

	"github.com/cashshuffle/cashshuffle/message"
)

// broadcastMessage processes messages and either broadcasts
// them to all connected users, or to a single user.
func (sc *signedConn) broadcastMessage() error {
	to := sc.message.GetPacket().GetTo().String()

	if to == "" {
		err := sc.broadcastAll()
		if err != nil {
			sc.broadcastNewRound()

			// Don't disconnect, we broadcasted a new round.
			return nil
		}
	} else {
		td := sc.tracker.getVerificationKeyData(to)
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
	for conn := range sc.tracker.connections {
		if conn == sc.conn {
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
	for conn := range sc.tracker.connections {
		if conn == sc.conn {
			continue
		}

		td := sc.tracker.getTrackerData(conn)

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
