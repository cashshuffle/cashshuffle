package server

import (
	"fmt"
	"strings"

	"github.com/cashshuffle/cashshuffle/message"

	log "github.com/sirupsen/logrus"
)

var (
	playerPrefix = "Player"
	broadcastKey = "Broadcast"
)

// broadcastMessage processes messages and either broadcasts
// them to all connected users, or to a single user.
func (pi *packetInfo) broadcastMessage() {
	msgMap := make(map[string][]*message.Signed)

	for _, signed := range pi.message.GetPacket() {
		to := signed.GetPacket().GetToKey()

		if to == nil {
			msgMap[broadcastKey] = append(msgMap[broadcastKey], signed)
		} else {
			k := fmt.Sprintf("%s%s", playerPrefix, signed.GetPacket().GetToKey().GetKey())
			msgMap[k] = append(msgMap[k], signed)
		}
	}

	for vk, msgs := range msgMap {
		if vk == broadcastKey {
			pi.broadcastAll(msgs)
		} else {
			sendingPlayer := pi.tracker.playerByConnection(pi.conn)
			if sendingPlayer == nil {
				log.Debug(logDirectMessage + "Sending message from disconnected player\n")
			}

			player := pi.tracker.playerByVerificationKey(strings.TrimLeft(vk, playerPrefix))
			if player == nil {
				log.Debugf(logDirectMessage+"Ignoring message to vk:%s because player has disconnected\n", vk)
				return
			}

			if player == sendingPlayer {
				log.Debugf(logDirectMessage+"Ignoring message to self\nPlayer: %s\n", sendingPlayer)
				return
			}

			log.Debugf(logDirectMessage+"From: %s\n", sendingPlayer)
			log.Debugf(logDirectMessage+"To: %s\n", player)

			// stop sending messages after the first error
			if err := writeMessage(player.conn, msgs); err != nil {
				log.Debugf(logDirectMessage+"Error writing message: %s\n", err)
				return
			}
		}
	}
}

// broadcastAll broadcasts to all participants.
func (pi *packetInfo) broadcastAll(msgs []*message.Signed) {
	pi.tracker.mutex.RLock()
	defer pi.tracker.mutex.RUnlock()

	sender := pi.tracker.connections[pi.conn]
	if sender == nil {
		log.Debugf(logBroadcast+"Ignoring message from %s because player has disconnected\n", getIP(pi.conn))
		return
	}

	log.Debugf(logBroadcast+"From: %s\n", sender)

	for _, player := range sender.pool.players {
		// Try to send the message to remaining players even if errors.
		if err := writeMessage(player.conn, msgs); err != nil {
			log.Debugf(logBroadcast+"Continuing to send after write error: %s\nTo: %s\n", err, player)
		}
	}
}

// announceStart sends an announcement message if the pool
// is full.
func (pi *packetInfo) announceStart() {
	pi.tracker.mutex.RLock()
	defer pi.tracker.mutex.RUnlock()

	sender := pi.tracker.connections[pi.conn]

	// If the user has disconnected, then no need to send
	// the broadcast.
	if sender == nil {
		log.Debugf(logPhaseAnnounce+"Ignoring message from %s because player has disconnected\n", getIP(pi.conn))
		return
	}

	announcement := []*message.Signed{
		{
			Packet: &message.Packet{
				Phase:  message.Phase_ANNOUNCEMENT,
				Number: uint32(pi.tracker.poolSize),
			},
		},
	}

	for _, player := range sender.pool.players {
		// The player now has an obligation to send verification key.
		// Since we cannot differentiate between a user ignoring the message
		// and an honest miss, we assume the user always receives the message.
		player.isPassive = true

		// Try to send the message to remaining players even if errors.
		if err := writeMessage(player.conn, announcement); err != nil {
			log.Debugf(logBroadcast+"Continuing to send after write error: %s\nTo: %s\n", err, player)
		}
	}
}

func (pi *packetInfo) broadcastJoinedPool(p *PlayerData) {
	m := message.Signed{
		Packet: &message.Packet{
			Number: p.number,
		},
	}

	pi.broadcastAll([]*message.Signed{&m})
}
