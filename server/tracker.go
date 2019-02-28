package server

import (
	"net"
	"sync"
	"time"

	"github.com/cashshuffle/cashshuffle/message"
	"github.com/nats-io/nuid"
)

const (
	// banTime is the amount of time to ban an IP.
	banTime = 30 * time.Minute

	// banScoreTick is the ban score increment on each pool ban.
	banScoreTick = 1

	// maxBanScore is the score the connection much reach to
	// be banned by IP.
	maxBanScore = 3
)

// Tracker is used to track connections to the server.
type Tracker struct {
	bannedIPs            map[string]*banData
	connections          map[net.Conn]*trackerData
	verificationKeys     map[string]net.Conn
	mutex                sync.RWMutex
	pools                map[int]map[uint32]interface{}
	poolAmounts          map[int]uint64
	poolVersions         map[int]uint64
	poolTypes            map[int]message.ShuffleType
	poolSize             int
	fullPools            map[int]interface{}
	shufflePort          int
	shuffleWebSocketPort int
}

// banData is the data required to track IP bans.
type banData struct {
	score uint32
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
	version         uint64
	shuffleType     message.ShuffleType
}

// NewTracker instantiates a new tracker
func NewTracker(poolSize int, shufflePort int, shuffleWebSocketPort int) *Tracker {
	return &Tracker{
		poolSize:             poolSize,
		bannedIPs:            make(map[string]*banData),
		connections:          make(map[net.Conn]*trackerData),
		verificationKeys:     make(map[string]net.Conn),
		pools:                make(map[int]map[uint32]interface{}),
		poolAmounts:          make(map[int]uint64),
		poolVersions:         make(map[int]uint64),
		poolTypes:            make(map[int]message.ShuffleType),
		fullPools:            make(map[int]interface{}),
		shufflePort:          shufflePort,
		shuffleWebSocketPort: shuffleWebSocketPort,
	}
}

// add adds a connection to the tracker.
func (t *Tracker) add(data *trackerData) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.verificationKeys[data.verificationKey] = data.conn

	data.sessionID = t.generateSessionID()

	t.connections[data.conn] = data

	data.pool, data.number = t.assignPool(data)
}

// remove removes the connection.
func (t *Tracker) remove(conn net.Conn) {
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
func (t *Tracker) banned(data *trackerData) bool {
	return t.poolSize == (len(data.bannedBy) + 1)
}

// count returns the number of connections to the server.
func (t *Tracker) count() int {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return len(t.connections)
}

// bannedIP returns true if the player has been banned from the server.
func (t *Tracker) bannedIP(conn net.Conn) bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	banData := t.bannedIPs[ip]
	if banData != nil && banData.score >= maxBanScore {
		return true
	}

	return false
}

// banIP bans an IP on the server.
func (t *Tracker) banIP(conn net.Conn) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	if _, ok := t.bannedIPs[ip]; ok {
		t.bannedIPs[ip].score += banScoreTick
	} else {
		t.bannedIPs[ip] = &banData{
			score: banScoreTick,
		}
	}

	go t.cleanupBan(ip)
}

// cleanupBan is the decrementer on the ban score and
// cleans up IPs that no longer need to be tracked.
func (t *Tracker) cleanupBan(ip string) {
	time.Sleep(banTime)

	t.mutex.Lock()
	defer t.mutex.Unlock()

	if _, ok := t.bannedIPs[ip]; ok {
		t.bannedIPs[ip].score -= banScoreTick
	}

	if t.bannedIPs[ip].score == 0 {
		delete(t.bannedIPs, ip)
	}
}

// getVerifcationKeyConn gets the connection for a verification key.
func (t *Tracker) getVerificationKeyData(key string) *trackerData {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if _, ok := t.verificationKeys[key]; ok {
		return t.connections[t.verificationKeys[key]]
	}

	return nil
}

// getTrackerdData returns trackerdata associated with a connection.
func (t *Tracker) getTrackerData(c net.Conn) *trackerData {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.connections[c]
}

// getPoolSize returns the pool size for the connection.
func (t *Tracker) getPoolSize(pool int) int {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return len(t.pools[pool])
}

// generateSessionID generates a unique session id.
// This method assumes the caller is holding the mutex.
func (t *Tracker) generateSessionID() []byte {
	n := nuid.New()

	return []byte(n.Next())
}

// assignPool assigns a user to a pool.
// This method assumes the caller is holding the mutex.
func (t *Tracker) assignPool(data *trackerData) (int, uint32) {
	num := 1

	for {
		if _, ok := t.pools[num]; ok {
			if t.poolAmounts[num] != data.amount {
				num = num + 1
				continue
			}

			if t.poolVersions[num] != data.version {
				num = num + 1
				continue
			}

			if t.poolTypes[num] != data.shuffleType {
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
		t.poolVersions[num] = data.version
		t.poolTypes[num] = data.shuffleType
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
func (t *Tracker) unassignPool(td *trackerData) {
	delete(t.pools[td.pool], td.number)

	if len(t.pools[td.pool]) == 0 {
		delete(t.pools, td.pool)
		delete(t.fullPools, td.pool)
		delete(t.poolAmounts, td.pool)
		delete(t.poolVersions, td.pool)
		delete(t.poolTypes, td.pool)
	}
}
