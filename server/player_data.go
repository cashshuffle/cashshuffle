package server

import (
	"fmt"
	"net"
	"sync"

	"github.com/cashshuffle/cashshuffle/message"
)

// PlayerData is data needed about each connection.
type PlayerData struct {
	mutex           sync.RWMutex
	sessionID       []byte
	number          uint32
	conn            net.Conn
	verificationKey string
	pool            *Pool
	blamedBy        map[string]interface{}
	amount          uint64
	version         uint64
	shuffleType     message.ShuffleType
	isPassive       bool
}

// addBlame adds a verification key to the blamedBy map.
func (p *PlayerData) addBlame(verificationKey string) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, ok := p.blamedBy[verificationKey]; ok {
		return false
	}

	p.blamedBy[verificationKey] = nil

	return true
}

func (p *PlayerData) String() string {
	return fmt.Sprintf("("+
		"vk:%s, "+
		"pool:%d, "+
		"num:%d, "+
		"ip:%s"+
		")",
		p.verificationKey, p.pool.num, p.number, getIP(p.conn))
}
