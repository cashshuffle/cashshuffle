package server

import "errors"

// verifyMessage makes sure all required fields exist.
func (sc *signedConn) verifyMessage() error {
	td := sc.tracker.getTrackerData(sc.conn)

	if string(sc.message.GetPacket().GetSession()) != string(td.sessionID) {
		return errors.New("invalid session")
	}

	if sc.message.GetPacket().GetFrom().String() != td.verificationKey {
		return errors.New("invalid verification key")
	}

	if sc.message.GetPacket().GetNumber() != td.number {
		return errors.New("invalid user number")
	}

	to := sc.message.GetPacket().GetTo().String()
	if to != "" {
		if sc.tracker.getVerificationKeyData(to) == nil {
			return errors.New("invalid destination")
		}
	}

	return nil
}
