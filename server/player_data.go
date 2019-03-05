package server

import (
	"net"
	"sync"

	"github.com/cashshuffle/cashshuffle/message"
)

// playerData is data needed about each connection.
type playerData struct {
	mutex           sync.RWMutex
	sessionID       []byte
	number          uint32
	conn            net.Conn
	verificationKey string
	pool            int
	blamedBy        map[string]interface{}
	amount          uint64
	version         uint64
	shuffleType     message.ShuffleType
}

// addBlame adds a verification key to the blamedBy map.
func (p *playerData) addBlame(verificationKey string) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, ok := p.blamedBy[verificationKey]; ok {
		return false
	}

	p.blamedBy[verificationKey] = nil

	return true
}
