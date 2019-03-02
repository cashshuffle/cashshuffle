package server

import (
	"errors"

	"github.com/cashshuffle/cashshuffle/message"
)

// checkBlameMessage checks if the player has sent a blame and handles it.
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
		blamedKey := packet.Message.Blame.Accused.String()
		blamed := pi.tracker.getVerificationKeyPlayer(blamedKey)

		if blamed == nil {
			return errors.New("invalid blame")
		}

		player := pi.tracker.getPlayerData(pi.conn)
		if player == nil {
			return nil
		}

		blamed.mutex.Lock()
		blamed.blamedBy[player.verificationKey] = nil
		blamed.mutex.Unlock()

		if pi.tracker.bannedByPool(blamed) {
			pi.tracker.increaseBanScore(blamed.conn)
			pi.tracker.decreaseVoters(player.pool)
		}

		return errors.New(blamedString)
	}

	return nil
}
