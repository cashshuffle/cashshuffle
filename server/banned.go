package server

import (
	"errors"

	"github.com/cashshuffle/cashshuffle/message"
)

const (
	playerBannedErrorMessage = "player banned"
)

// checkBanMessage checks to see if the player has sent a ban.
func (pi *packetInfo) checkBanMessage() error {
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
		banKey := packet.Message.Blame.Accused.String()
		banned := pi.tracker.playerByVerificationKey(banKey)

		if banned == nil {
			return nil
		}

		blamer := pi.tracker.playerByConnection(pi.conn)
		if blamer == nil {
			return nil
		}

		if banned.pool != blamer.pool {
			return errors.New("invalid ban")
		}

		added := banned.addBannedBy(blamer.verificationKey)
		if !added {
			return nil
		}

		if pi.tracker.banned(banned) {
			pi.tracker.banIP(banned.conn)
			pi.tracker.decreasePoolSize(blamer.pool)
		}

		return errors.New(playerBannedErrorMessage)
	}

	return nil
}
