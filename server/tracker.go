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
	mutex            sync.Mutex
	pools            map[int]map[uint32]interface{}
	poolAmounts      map[int]uint64
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
	bannedBy        map[string]interface{}
	amount          uint64
}

// init initializes the tracker.
func (t *tracker) init() {
	t.connections = make(map[net.Conn]*trackerData)
	t.verificationKeys = make(map[string]net.Conn)
	t.pools = make(map[int]map[uint32]interface{})
	t.poolAmounts = make(map[int]uint64)
	t.fullPools = make(map[int]interface{})
}

// add adds a connection to the tracker.
func (t *tracker) add(data *trackerData) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.verificationKeys[data.verificationKey] = data.conn

	data.sessionID = t.generateSessionID()

	t.connections[data.conn] = data

	data.pool, data.number = t.assignPool(data)
}

// remove removes the connection.
func (t *tracker) remove(conn net.Conn) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	td := t.connections[conn]
	if td != nil {
		if td.verificationKey != "" {
			delete(t.verificationKeys, td.verificationKey)
		}

		t.unassignPool(td)

		delete(t.connections, conn)
	}
}

// banned returns true if the player has been banned.
func (t *tracker) banned(data *trackerData) bool {
	return t.poolSize == (len(data.bannedBy) - 1)
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

	return len(t.pools[pool])
}

// generateSessionID generates a unique session id.
// This method assumes the caller is holding the mutex.
func (t *tracker) generateSessionID() []byte {
	n := nuid.New()

	return []byte(n.Next())
}

// assignPool assigns a user to a pool.
// This method assumes the caller is holding the mutex.
func (t *tracker) assignPool(data *trackerData) (int, uint32) {
	num := 1

	for {
		if _, ok := t.pools[num]; ok {
			if t.poolAmounts[num] != data.amount {
				num = num + 1
				continue
			}

			if _, ok := t.fullPools[num]; ok {
				num = num + 1
				continue
			}
		}

		break
	}

	playerNum := uint32(1)
	if _, ok := t.pools[num]; !ok {
		t.pools[num] = make(map[uint32]interface{})
		t.pools[num][1] = nil
		t.poolAmounts[num] = data.amount
	} else {
		for {
			if _, ok := t.pools[num][playerNum]; ok {
				playerNum = playerNum + 1
				continue
			}

			break
		}

		t.pools[num][playerNum] = nil
	}

	if len(t.pools[num]) == t.poolSize {
		t.fullPools[num] = nil
	}

	return num, playerNum
}

// unassignPool removes a user from a pool.
// This method assumes the caller is holding the mutex.
func (t *tracker) unassignPool(td *trackerData) {
	delete(t.pools[td.pool], td.number)

	if len(t.pools[td.pool]) == 0 {
		delete(t.pools, td.pool)
		delete(t.fullPools, td.pool)
		delete(t.poolAmounts, td.pool)
	}
}
