package server

import (
	"errors"
	"fmt"

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

	if packet.GetMessage().GetBlame() == nil {
		return nil
	}

	validBlame := false

	for _, reason := range validBlamereasons {
		if packet.GetMessage().GetBlame().GetReason() == reason {
			validBlame = true
		}
	}

	if validBlame {
		blamer := pi.tracker.playerByConnection(pi.conn)
		if blamer == nil {
			return nil
		}
		accusedKey := packet.GetMessage().GetBlame().GetAccused().GetKey()
		accused := blamer.pool.PlayerFromSnapshot(accusedKey)
		if accused == nil {
			return errors.New("invalid blame")
		}

		if accused.pool != blamer.pool {
			return errors.New("invalid blame")
		}

		// After validating everything, we can skip the actual ban
		// if the pool already has banned someone.
		if blamer.pool.firstBan != nil {
			return nil
		}

		added := accused.addBlame(blamer.verificationKey)
		if !added {
			return nil
		}

		if blamer.pool.IsBanned(accused) {
			blamer.pool.firstBan = accused
			pi.tracker.increaseBanScore(accused.conn, false)
			if debugMode {
				fmt.Printf("[DenyIP] User blamed out of round: %s\n", accused.verificationKey)
			}
			pi.tracker.addDenyIPMatch(accused.conn, accused.pool, false)
		}
	}

	return nil
}
