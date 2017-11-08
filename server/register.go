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

			if p.From.String() != "" {
				td := trackerData{
					verificationKey: p.From.String(),
					conn:            pi.conn,
				}
				pi.tracker.add(&td)

				err := pi.registrationSuccess(&td)
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
