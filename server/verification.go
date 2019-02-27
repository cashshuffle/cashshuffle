package server

import (
	"errors"
)

// verifyMessage makes sure all required fields exist.
func (pi *packetInfo) verifyMessage() error {
	td := pi.tracker.getTrackerData(pi.conn)
	if td == nil {
		return errors.New("player disconnected")
	}

	for _, pkt := range pi.message.Packet {
		packet := pkt.GetPacket()

		if string(packet.GetSession()) != string(td.sessionID) {
			return errors.New("invalid session")
		}

		if packet.GetFromKey().String() != td.verificationKey {
			return errors.New("invalid verification key")
		}

		if pi.tracker.banned(td) {
			return errors.New("banned player")
		}

		if packet.GetNumber() != td.number {
			return errors.New("invalid user number")
		}

		to := packet.GetToKey()
		if to != nil {
			if pi.tracker.getVerificationKeyData(to.String()) == nil {
				return errors.New("invalid destination")
			}
		}
	}

	return nil
}
