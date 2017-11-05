package server

import (
	"net"
	"sync"

	"github.com/nats-io/nuid"
)

// tracker is used to track connections to the server.
type tracker struct {
	connections      map[net.Conn]*trackerData
	verificationKeys map[string]net.Conn
	playerNumbers    map[uint32]interface{}
	mutex            sync.Mutex
	pools            map[int]int
	poolSize         int
	fullPools        map[int]interface{}
}

// trackerData is data needed about each connection.
type trackerData struct {
	mutex           sync.Mutex
	sessionID       []byte
	number          uint32
	conn            net.Conn
	verificationKey string
	pool            int
}

// init initializes the tracker.
func (t *tracker) init() {
	t.connections = make(map[net.Conn]*trackerData)
	t.verificationKeys = make(map[string]net.Conn)
	t.playerNumbers = make(map[uint32]interface{})
	t.pools = make(map[int]int)
	t.fullPools = make(map[int]interface{})

	return
}

// add adds a connection to the tracker.
func (t *tracker) add(data *trackerData) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.verificationKeys[data.verificationKey] = data.conn

	data.number = t.generateNumber()
	data.sessionID = t.generateSessionID()

	t.connections[data.conn] = data

	data.pool = t.assignPool()

	return
}

// remove removes the connection.
func (t *tracker) remove(conn net.Conn) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	td := t.connections[conn]
	if td != nil {
		if td.number != 0 {
			delete(t.playerNumbers, td.number)
		}

		if td.verificationKey != "" {
			delete(t.verificationKeys, td.verificationKey)
		}

		t.unassignPool(td)

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

// getVerifcationKeyConn gets the connection for a verification key.
func (t *tracker) getVerificationKeyData(key string) *trackerData {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if _, ok := t.verificationKeys[key]; ok {
		return t.connections[t.verificationKeys[key]]
	}

	return nil
}

// getTrackerdData returns trackerdata associated with a connection.
func (t *tracker) getTrackerData(c net.Conn) *trackerData {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.connections[c]
}

// getPoolSize returns the pool size for the connection.
func (t *tracker) getPoolSize(pool int) int {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.pools[pool]
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

// assignPool assigns a user to a pool.
// This method assumes the caller is holding the mutex.
func (t *tracker) assignPool() int {
	var members int
	num := 1

	for {
		if v, ok := t.pools[num]; ok {
			if _, ok := t.fullPools[num]; ok {
				num = num + 1
				continue
			}

			members = v + 1
		} else {
			members = 1
		}

		break
	}

	t.pools[num] = members

	if members == t.poolSize {
		t.fullPools[num] = nil
	}

	return num
}

// unassignPool removes a user from a pool.
// This method assumes the caller is holding the mutex.
func (t *tracker) unassignPool(td *trackerData) {
	t.pools[td.pool] = t.pools[td.pool] - 1

	if t.pools[td.pool] == 0 {
		delete(t.pools, td.pool)
		delete(t.fullPools, td.pool)
	}

	return
}
