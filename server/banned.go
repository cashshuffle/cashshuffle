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

		player := pi.tracker.getPlayerData(pi.conn)
		if player == nil {
			return nil
		}

		bannedTrackerData.mutex.Lock()
		bannedTrackerData.bannedBy[player.verificationKey] = nil
		bannedTrackerData.mutex.Unlock()

		if pi.tracker.banned(bannedTrackerData) {
			pi.tracker.banIP(bannedTrackerData.conn)
			pi.tracker.decreasePoolSize(player.pool)
		}

		return errors.New("player banned")
	}

	return nil
}
