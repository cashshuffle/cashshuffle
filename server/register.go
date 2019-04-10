package server

import (
	"errors"
	"fmt"

	"github.com/cashshuffle/cashshuffle/message"
)

// registerClient registers a new session.
func (pi *packetInfo) registerClient() error {
	var player *PlayerData
	if len(pi.message.Packet) == 1 {
		signed := pi.message.Packet[0]

		if signed.GetSignature() == nil {
			p := signed.GetPacket()
			registration := p.GetRegistration()

			verificationKey := p.GetFromKey().GetKey()
			player = pi.tracker.playerByVerificationKey(verificationKey)
			if player != nil {
				return fmt.Errorf("server already has a player "+
					"with verification key %s", verificationKey)
			}

			if verificationKey != "" && registration != nil {
				player = &PlayerData{
					verificationKey: verificationKey,
					conn:            pi.conn,
					blamedBy:        make(map[string]interface{}),
					amount:          registration.GetAmount(),
					shuffleType:     registration.GetType(),
					version:         registration.GetVersion(),
					isPassive:       false,
				}
				pi.tracker.add(player)

				err := pi.registrationSuccess(player)
				if err != nil {
					pi.tracker.remove(pi.conn)
				}

				return err
			}
		}
	}

	if err := pi.registrationFailed(); err != nil {
		return err
	}

	return errors.New("registration failed")
}

// registrationSuccess sends a registration success reply.
func (pi *packetInfo) registrationSuccess(p *PlayerData) error {
	m := message.Signed{
		Packet: &message.Packet{
			Session: p.sessionID,
			Number:  p.number,
		},
	}

	return writeMessage(pi.conn, []*message.Signed{&m})
}

// registrationFailed sends a registration failed reply.
func (pi *packetInfo) registrationFailed() error {
	m := message.Signed{
		Packet: &message.Packet{
			Message: &message.Message{
				Blame: &message.Blame{
					Reason: message.Reason_INVALIDFORMAT,
				},
			},
		},
	}

	return writeMessage(pi.conn, []*message.Signed{&m})
}
