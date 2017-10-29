package server

import (
	"net"
	"sync"

	"github.com/nats-io/nuid"
)

// tracker is used to track connections to the server.
type tracker struct {
	connections           map[net.Conn]*trackerData
	verificationKeyLookup map[string]net.Conn
	playerNumbers         map[uint32]interface{}
	mutex                 sync.Mutex
}

// trackerData is data needed about each connection.
type trackerData struct {
	mutex           sync.Mutex
	sessionID       []byte
	number          uint32
	conn            net.Conn
	verificationKey string
}

// init initializes the tracker.
func (t *tracker) init() {
	t.connections = make(map[net.Conn]*trackerData)
	t.verificationKeyLookup = make(map[string]net.Conn)

	return
}

// add adds a connection to the tracker.
func (t *tracker) add(data *trackerData) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.verificationKeyLookup[data.verificationKey] = data.conn

	data.number = t.generateNumber()
	data.sessionID = t.generateSessionID()

	t.connections[data.conn] = data

	return
}

// remove removes the connection.
func (t *tracker) remove(conn net.Conn) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.connections[conn] != nil {
		if t.connections[conn].verificationKey != "" {
			delete(t.verificationKeyLookup, t.connections[conn].verificationKey)
		}

		delete(t.connections, conn)
	}

	return
}

// count returns the number of connections to the server.
func (t *tracker) count() int {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return len(t.connections)
}

// getTrackerdData returns trackerdata associated with a connection
func (t *tracker) getTrackerData(c net.Conn) *trackerData {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.connections[c]
}

// generateNumber gets the connections number in the pool.
// This method assumes the caller is holding the mutex.
func (t *tracker) generateNumber() uint32 {
	num := uint32(1)

	for {
		if _, ok := t.playerNumbers[num]; ok {
			num = num + 1
			continue
		}

		break
	}

	t.playerNumbers[num] = nil
	return num
}

// generateSessionID generates a unique session id.
// This method assumes the caller is holding the mutex.
func (t *tracker) generateSessionID() []byte {
	n := nuid.New()

	return []byte(n.Next())
}
