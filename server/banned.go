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
		banKey := packet.Message.Blame.Accused.Key

		bannedTrackerData := pi.tracker.getVerificationKeyData(banKey)
		if bannedTrackerData == nil {
			return errors.New("invalid ban")
		}

		td := pi.tracker.getTrackerData(pi.conn)
		if td == nil {
			return nil
		}

		pi.tracker.mutex.Lock()
		defer pi.tracker.mutex.Unlock()

		bannedTrackerData.bannedBy[td.verificationKey] = nil

		return nil
	}

	return nil
}
