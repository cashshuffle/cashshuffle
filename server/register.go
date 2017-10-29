package server

import (
	"errors"

	"github.com/cashshuffle/cashshuffle/message"
	"github.com/nats-io/nuid"
)

// registerClient registers a new session.
func (sc *signedConn) registerClient() error {
	if sc.message.GetSignature().String() == "" {
		p := sc.message.GetPacket()
		if p.From.String() != "" {
			n := nuid.New()

			td := trackerData{
				verificationKey: p.From.String(),
				sessionID:       []byte(n.Next()),
				conn:            sc.conn,
				number:          0,
			}
			sc.tracker.add(sc.conn, &td)

			err := sc.registrationSuccess(&td)
			return err
		}
	}

	if err := sc.registrationFailed(); err != nil {
		return err
	}

	return errors.New("registration failed")
}

// registrationSuccess sends a registration success reply.
func (sc *signedConn) registrationSuccess(td *trackerData) error {
	m := message.Signed{
		Packet: &message.Packet{
			Session: td.sessionID,
			Number:  td.number,
		},
	}

	err := writeMessage(sc.conn, &m)
	return err
}

// registrationFailed sends a registration failed reply.
func (sc *signedConn) registrationFailed() error {
	m := message.Signed{
		Packet: &message.Packet{
			Message: &message.Message{
				Blame: &message.Blame{
					Reason: message.Reason_INVALIDFORMAT,
				},
			},
		},
	}

	err := writeMessage(sc.conn, &m)
	return err
}
