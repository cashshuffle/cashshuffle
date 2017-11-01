package server

import "errors"

// verifyMessage makes sure all required fields exist.
func (sc *signedConn) verifyMessage() error {
	td := sc.tracker.getTrackerData(sc.conn)

	packet := sc.message.GetPacket()

	if string(packet.GetSession()) != string(td.sessionID) {
		return errors.New("invalid session")
	}

	if packet.GetFrom().String() != td.verificationKey {
		return errors.New("invalid verification key")
	}

	if packet.GetNumber() != td.number {
		return errors.New("invalid user number")
	}

	to := packet.GetTo()
	if to != nil {
		if sc.tracker.getVerificationKeyData(to.String()) == nil {
			return errors.New("invalid destination")
		}
	}

	return nil
}
