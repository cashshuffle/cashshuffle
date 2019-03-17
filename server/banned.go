package server

import (
	"errors"

	"github.com/cashshuffle/cashshuffle/message"
)

var validBlamereasons = []message.Reason{
	message.Reason_LIAR,
	message.Reason_INSUFFICIENTFUNDS,
	message.Reason_DOUBLESPEND,
	message.Reason_EQUIVOCATIONFAILURE,
	message.Reason_SHUFFLEFAILURE,
	message.Reason_SHUFFLEANDEQUIVOCATIONFAILURE,
	message.Reason_MISSINGOUTPUT,
	message.Reason_INVALIDSIGNATURE,
	message.Reason_INVALIDFORMAT,
}

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

	validBlame := false

	for _, reason := range validBlamereasons {
		if packet.Message.Blame.Reason == reason {
			validBlame = true
		}
	}

	if validBlame {
		blamer := pi.tracker.playerByConnection(pi.conn)
		if blamer == nil {
			return nil
		}

		accusedKey := packet.Message.Blame.Accused.String()
		players := pi.tracker.blameablePlayers(blamer.pool)
		accused := players[accusedKey]
		if accused == nil {
			return errors.New("invalid blame")
		}

		if accused.pool != blamer.pool {
			return errors.New("invalid blame")
		}

		added := accused.addBlame(blamer.verificationKey)
		if !added {
			return nil
		}

		if pi.tracker.bannedByPool(accused, true) {
			pi.tracker.increaseBanScore(accused.conn)
			pi.tracker.decreasePoolVoters(accused.pool)
			pi.tracker.addDenyIPMatch(accused.conn, accused.pool)
		}
	}

	return nil
}
