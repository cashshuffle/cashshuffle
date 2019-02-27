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
				td := trackerData{
					verificationKey: p.FromKey.String(),
					conn:            pi.conn,
					bannedBy:        make(map[string]interface{}),
					amount:          registration.GetAmount(),
					shuffleType:     registration.GetType(),
					version:         registration.GetVersion(),
				}
				pi.tracker.add(&td)

				err := pi.registrationSuccess(&td)
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
func (pi *packetInfo) registrationSuccess(td *trackerData) error {
	m := message.Signed{
		Packet: &message.Packet{
			Session: td.sessionID,
			Number:  td.number,
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
