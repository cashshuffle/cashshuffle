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
	bannedIPs               map[string]*banData
	connections             map[net.Conn]*playerData
	verificationKeys        map[string]net.Conn
	mutex                   sync.RWMutex
	pools                   map[int]map[uint32]*playerData
	poolAmounts             map[int]uint64
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

// playerData is data needed about each connection.
type playerData struct {
	mutex           sync.RWMutex
	sessionID       []byte
	number          uint32
	conn            net.Conn
	verificationKey string
	pool            int
	poolSize        int
	bannedBy        map[string]interface{}
	amount          uint64
	version         uint64
	shuffleType     message.ShuffleType
}

// NewTracker instantiates a new tracker
func NewTracker(poolSize int, shufflePort int, shuffleWebSocketPort int, torShufflePort int, torShuffleWebSocketPort int) *Tracker {
	return &Tracker{
		poolSize:                poolSize,
		bannedIPs:               make(map[string]*banData),
		connections:             make(map[net.Conn]*playerData),
		verificationKeys:        make(map[string]net.Conn),
		pools:                   make(map[int]map[uint32]*playerData),
		poolAmounts:             make(map[int]uint64),
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
func (t *Tracker) add(data *playerData) {
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

	player := t.connections[conn]
	if player != nil {
		if player.verificationKey != "" {
			delete(t.verificationKeys, player.verificationKey)
		}

		t.unassignPool(player)

		delete(t.connections, conn)
	}
}

// banned returns true if the player has been banned.
func (t *Tracker) banned(data *playerData) bool {
	data.mutex.RLock()
	defer data.mutex.RUnlock()

	return data.poolSize <= (len(data.bannedBy) + 1)
}

// count returns the number of connections to the server.
func (t *Tracker) count() int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return len(t.connections)
}

// bannedIP returns true if the player has been banned from the server.
func (t *Tracker) bannedIP(conn net.Conn) bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

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
func (t *Tracker) getVerificationKeyData(key string) *playerData {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if _, ok := t.verificationKeys[key]; ok {
		return t.connections[t.verificationKeys[key]]
	}

	return nil
}

// getPlayerData returns playerData associated with a connection.
func (t *Tracker) getPlayerData(c net.Conn) *playerData {
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
func (t *Tracker) assignPool(player *playerData) (int, uint32) {
	num := 1

	for {
		if _, ok := t.pools[num]; ok {
			if t.poolAmounts[num] != player.amount {
				num = num + 1
				continue
			}

			if t.poolVersions[num] != player.version {
				num = num + 1
				continue
			}

			if t.poolTypes[num] != player.shuffleType {
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
		t.pools[num][1] = player
		t.poolAmounts[num] = player.amount
		t.poolVersions[num] = player.version
		t.poolTypes[num] = player.shuffleType
	} else {
		for {
			if _, ok := t.pools[num][playerNum]; ok {
				playerNum = playerNum + 1
				continue
			}

			break
		}

		t.pools[num][playerNum] = player
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

	for _, player := range t.pools[pool] {
		if player == nil {
			continue
		}

		player.mutex.Lock()
		defer player.mutex.Unlock()

		player.poolSize--
	}
}

// unassignPool removes a user from a pool.
// This method assumes the caller is holding the mutex.
func (t *Tracker) unassignPool(player *playerData) {
	delete(t.pools[player.pool], player.number)

	if len(t.pools[player.pool]) == 0 {
		delete(t.pools, player.pool)
		delete(t.fullPools, player.pool)
		delete(t.poolAmounts, player.pool)
		delete(t.poolVersions, player.pool)
		delete(t.poolTypes, player.pool)
	}
}
