package server

import (
	"net"
	"sync"
	"time"

	"github.com/nats-io/nuid"

	"github.com/cashshuffle/cashshuffle/message"
)

const (
	// banTime is the amount of time to ban an IP.
	banTime = 15 * time.Minute

	// banScoreTick is the ban score increment on each pool ban.
	banScoreTick = 1

	// maxBanScore is the score the connection much reach to
	// be banned by IP.
	maxBanScore = 3
)

// Tracker is used to track connections to the server.
type Tracker struct {
	banDatas                map[string]*banData
	connections             map[net.Conn]*playerData
	verificationKeys        map[string]net.Conn
	mutex                   sync.RWMutex
	pools                   map[int]map[uint32]*playerData
	poolAmounts             map[int]uint64
	poolSizes               map[int]int
	poolVersions            map[int]uint64
	poolTypes               map[int]message.ShuffleType
	poolSize                int
	fullPools               map[int]interface{}
	shufflePort             int
	shuffleWebSocketPort    int
	torShufflePort          int
	torShuffleWebSocketPort int
}

// banData is the data required to track IP bans.
type banData struct {
	score uint32
}

// NewTracker instantiates a new tracker
func NewTracker(poolSize int, shufflePort int, shuffleWebSocketPort int, torShufflePort int, torShuffleWebSocketPort int) *Tracker {
	return &Tracker{
		poolSize:                poolSize,
		banDatas:                make(map[string]*banData),
		connections:             make(map[net.Conn]*playerData),
		verificationKeys:        make(map[string]net.Conn),
		pools:                   make(map[int]map[uint32]*playerData),
		poolAmounts:             make(map[int]uint64),
		poolSizes:               make(map[int]int),
		poolVersions:            make(map[int]uint64),
		poolTypes:               make(map[int]message.ShuffleType),
		fullPools:               make(map[int]interface{}),
		shufflePort:             shufflePort,
		shuffleWebSocketPort:    shuffleWebSocketPort,
		torShufflePort:          torShufflePort,
		torShuffleWebSocketPort: torShuffleWebSocketPort,
	}
}

// add adds a connection to the tracker.
func (t *Tracker) add(p *playerData) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.verificationKeys[p.verificationKey] = p.conn

	p.sessionID = t.generateSessionID()

	t.connections[p.conn] = p

	p.pool, p.number = t.assignPool(p)
}

// remove removes the connection.
func (t *Tracker) remove(conn net.Conn) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	player := t.connections[conn]
	if player != nil {
		if player.verificationKey != "" {
			delete(t.verificationKeys, player.verificationKey)
		}

		t.unassignPool(player)

		delete(t.connections, conn)
	}
}

// bannedByPool returns true if the player has been banned by their pool.
func (t *Tracker) bannedByPool(p *playerData) bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.poolSizes[p.pool] <= (len(p.blamedBy) + 1)
}

// count returns the number of connections to the server.
func (t *Tracker) count() int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return len(t.connections)
}

// bannedByServer returns true if the player has been banned from the server.
func (t *Tracker) bannedByServer(conn net.Conn) bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	banData := t.banDatas[ip]
	if banData != nil && banData.score >= maxBanScore {
		return true
	}

	return false
}

// increaseBanScore increases the ban score for an IP on the server.
func (t *Tracker) increaseBanScore(conn net.Conn) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	if _, ok := t.banDatas[ip]; ok {
		t.banDatas[ip].score += banScoreTick
	} else {
		t.banDatas[ip] = &banData{
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

	if _, ok := t.banDatas[ip]; ok {
		t.banDatas[ip].score -= banScoreTick
	}

	if t.banDatas[ip].score == 0 {
		delete(t.banDatas, ip)
	}
}

// playerByVerificationKey gets the player for a verification key.
func (t *Tracker) playerByVerificationKey(key string) *playerData {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if _, ok := t.verificationKeys[key]; ok {
		return t.connections[t.verificationKeys[key]]
	}

	return nil
}

// playerByConnection gets the player for a connection.
func (t *Tracker) playerByConnection(c net.Conn) *playerData {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.connections[c]
}

// getPoolSize returns the pool size for the connection.
func (t *Tracker) getPoolSize(pool int) int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

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
func (t *Tracker) assignPool(p *playerData) (int, uint32) {
	num := 1

	for {
		if _, ok := t.pools[num]; ok {
			if t.poolAmounts[num] != p.amount {
				num = num + 1
				continue
			}

			if t.poolVersions[num] != p.version {
				num = num + 1
				continue
			}

			if t.poolTypes[num] != p.shuffleType {
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
		t.pools[num] = make(map[uint32]*playerData)
		t.pools[num][1] = p
		t.poolAmounts[num] = p.amount
		t.poolSizes[num] = t.poolSize
		t.poolVersions[num] = p.version
		t.poolTypes[num] = p.shuffleType
	} else {
		for {
			if _, ok := t.pools[num][playerNum]; ok {
				playerNum = playerNum + 1
				continue
			}

			break
		}

		t.pools[num][playerNum] = p
	}

	if len(t.pools[num]) == t.poolSize {
		t.fullPools[num] = nil
	}

	return num, playerNum
}

// decreasePoolSize decreases the pool size being
// tracked in playerData after a blame occurs.
func (t *Tracker) decreasePoolSize(pool int) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	t.poolSizes[pool]--
}

// unassignPool removes a user from a pool.
// This method assumes the caller is holding the mutex.
func (t *Tracker) unassignPool(p *playerData) {
	delete(t.pools[p.pool], p.number)

	if len(t.pools[p.pool]) == 0 {
		delete(t.pools, p.pool)
		delete(t.fullPools, p.pool)
		delete(t.poolAmounts, p.pool)
		delete(t.poolSizes, p.pool)
		delete(t.poolVersions, p.pool)
		delete(t.poolTypes, p.pool)
	}
}
