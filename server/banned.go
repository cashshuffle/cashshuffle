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
		bannedTrackerData := pi.tracker.getVerificationKeyData(banKey)

		if bannedTrackerData == nil {
			return nil
		}

		td := pi.tracker.getTrackerData(pi.conn)
		if td == nil {
			return nil
		}

		if bannedTrackerData.pool != td.pool {
			return errors.New("invalid ban")
		}

		added := bannedTrackerData.addBannedBy(td.verificationKey)
		if !added {
			return nil
		}

		if pi.tracker.banned(bannedTrackerData) {
			pi.tracker.banIP(bannedTrackerData.conn)
			pi.tracker.decreasePoolSize(td.pool)
		}

		return errors.New(playerBannedErrorMessage)
	}

	return nil
}
