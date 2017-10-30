package server

import "errors"

// broadcastMessage processes messages and either broadcasts
// them to all connected users, or to a single user.
func (sc *signedConn) broadcastMessage() error {
	to := sc.message.GetPacket().GetTo().String()

	if to == "" {
		err := sc.broadcastAll()
		if err != nil {
			return err
		}
	} else {
		td := sc.tracker.getVerificationKeyData(to)
		if td == nil {
			return errors.New("peer disconnected")
		}

		err := writeMessage(td.conn, sc.message)
		if err != nil {
			return err
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
