package server

import (
	"errors"

	"github.com/cashshuffle/cashshuffle/message"
)

// registerClient registers a new session.
func (pi *packetInfo) registerClient() error {
	if len(pi.message.Packet) == 1 {
		signed := pi.message.Packet[0]

		if signed.GetSignature() == nil {
			p := signed.GetPacket()
			registration := p.GetRegistration()

			if p.FromKey.String() != "" && registration != nil {
				player := playerData{
					verificationKey: p.FromKey.String(),
					conn:            pi.conn,
					blamedBy:        make(map[string]interface{}),
					amount:          registration.GetAmount(),
					shuffleType:     registration.GetType(),
					version:         registration.GetVersion(),
				}
				pi.tracker.add(&player)

				err := pi.registrationSuccess(&player)
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
func (pi *packetInfo) registrationSuccess(p *playerData) error {
	m := message.Signed{
		Packet: &message.Packet{
			Session: p.sessionID,
			Number:  p.number,
		},
	}

	err := writeMessage(pi.conn, []*message.Signed{&m})
	return err
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

	err := writeMessage(pi.conn, []*message.Signed{&m})
	return err
}
