package server

import (
	"errors"
	"fmt"

	"github.com/cashshuffle/cashshuffle/message"

	log "github.com/sirupsen/logrus"
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

	reason := packet.GetMessage().GetBlame().GetReason()
	validBlame := false

	for _, r := range validBlamereasons {
		if reason == r {
			validBlame = true
			break
		}
	}

	if !validBlame {
		return fmt.Errorf("unknown blame reason: %s", reason)
	}

	blamer := pi.tracker.playerByConnection(pi.conn)
	if blamer == nil {
		log.Debugf(logBlame+"Ignoring blame from %s because they have disconnected\n", getIP(pi.conn))
		return nil
	}
	accusedKey := packet.GetMessage().GetBlame().GetAccused().GetKey()
	accused := blamer.pool.PlayerFromSnapshot(accusedKey)
	if accused == nil {
		return errors.New("invalid blame - accused not in pool snapshot")
	}

	// After validating everything, we can skip the actual ban
	// if the pool already has banned someone.
	if blamer.pool.firstBan != nil {
		log.Debugf(logBlame+"Ignoring blame in pool %d because a player is already banned\n", blamer.pool.num)
		return nil
	}

	added := accused.addBlame(blamer.verificationKey)
	if !added {
		log.Debugf(logBlame+"Duplicate From: %s\n", blamer)
		log.Debugf(logBlame+"Duplicate To: %s\n", accused)
		return nil
	}

	log.Debugf(logBlame+"Blame applied for reason: %s\n", reason)
	log.Debugf(logBlame+"From: %s\n", blamer)
	log.Debugf(logBlame+"To: %s\n", accused)

	if blamer.pool.IsBanned(accused) {
		blamer.pool.firstBan = accused
		pi.tracker.increaseBanScore(accused.conn, false)
		log.Debugf(logBan+"User blamed out of round: %s\n", accused)
		pi.tracker.addDenyIPMatch(accused.conn, accused.pool, false)
	}

	return nil
}
