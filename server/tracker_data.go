package server

import (
	"net"
	"sync"

	"github.com/cashshuffle/cashshuffle/message"
)

// trackerData is data needed about each connection.
type trackerData struct {
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
func (td *trackerData) addBannedBy(verificationKey string) bool {
	td.mutex.Lock()
	defer td.mutex.Unlock()

	if _, ok := td.bannedBy[verificationKey]; ok {
		return false
	}

	td.bannedBy[verificationKey] = nil

	return true
}
