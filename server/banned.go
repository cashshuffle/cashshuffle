package server

import (
	"errors"

	"github.com/cashshuffle/cashshuffle/message"
)

// checkBlameMessage checks to see if the player has sent a blame.
func (pi *packetInfo) checkBlameMessage() error {
	if len(pi.message.Packet) != 1 {
		return nil
	}

	pkt := pi.message.Packet[0]
	packet := pkt.GetPacket()

	if packet.Message == nil {
		return nil
	}

	if packet.Message.Blame == nil {
		return nil
	}

	if packet.Message.Blame.Reason == message.Reason_LIAR {
		accusedKey := packet.Message.Blame.Accused.String()
		accused := pi.tracker.playerByVerificationKey(accusedKey)

		if accused == nil {
			return nil
		}

		blamer := pi.tracker.playerByConnection(pi.conn)
		if blamer == nil {
			return nil
		}

		if accused.pool != blamer.pool {
			return errors.New("invalid ban")
		}

		added := accused.addBlame(blamer.verificationKey)
		if !added {
			return nil
		}

		if pi.tracker.bannedByPool(accused) {
			pi.tracker.increaseBanScore(accused.conn)
			pi.tracker.decreasePoolVoters(blamer.pool)
			pi.tracker.addDenyIPMatch(blamer.conn, blamer.pool)
		}
	}

	return nil
}
