package server

import (
	"errors"

	"github.com/cashshuffle/cashshuffle/message"
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
			return errors.New("invalid ban")
		}

		td := pi.tracker.getTrackerData(pi.conn)
		if td == nil {
			return nil
		}

		bannedTrackerData.mutex.Lock()
		defer bannedTrackerData.mutex.Unlock()
		bannedTrackerData.bannedBy[td.verificationKey] = nil

		if pi.tracker.banned(bannedTrackerData) {
			pi.tracker.banIP(bannedTrackerData.conn)
		}

		return errors.New("player banned")
	}

	return nil
}
