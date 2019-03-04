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
	bannedBy        map[string]interface{}
	amount          uint64
	version         uint64
	shuffleType     message.ShuffleType
}

// addBannedBy adds a verification key to the bannedBy map.
func (p *playerData) addBannedBy(verificationKey string) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, ok := p.bannedBy[verificationKey]; ok {
		return false
	}

	p.bannedBy[verificationKey] = nil

	return true
}
