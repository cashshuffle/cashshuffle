package server

import (
	"net"
	"sync"
	"time"
	"fmt"

	"github.com/cashshuffle/cashshuffle/message"

	"github.com/nats-io/nuid"
)

const (
	// banTime is the amount of time to ban an IP.
	banTime = 15 * time.Minute

	// denyIPTime is the amount of time to avoid matching with other
	// IPs that have banned you from pools in the past.
	denyIPTime = 2 * time.Hour

	// banScoreTick is the ban score increment on each pool ban.
	banScoreTick = 1

	// maxBanScore is the score the connection much reach to
	// be banned by IP.
	maxBanScore = 5

	// firstPoolNum is the starting number for pools
	firstPoolNum = 1

	// firstPlayerNum is the starting number for players in a pool
	firstPlayerNum = uint32(1)
)

// Tracker is used to track connections to the server.
type Tracker struct {
	banData                 map[string]*banData
	connections             map[net.Conn]*playerData
	verificationKeys        map[string]net.Conn
	mutex                   sync.RWMutex
	denyIPMatch             map[ipPair]time.Time
	pools                   map[int]map[uint32]*playerData
	poolAmounts             map[int]uint64
	poolVoters              map[int]int
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

// ipPair is a canonically sorted pair of IPs
type ipPair struct {
	left  string
	right string
}

func newIPPair(a, b string) ipPair {
	if a < b {
		return ipPair{a, b}
	}
	return ipPair{b, a}
}

// NewTracker instantiates a new tracker
func NewTracker(poolSize int, shufflePort int, shuffleWebSocketPort int, torShufflePort int, torShuffleWebSocketPort int) *Tracker {
	return &Tracker{
		poolSize:                poolSize,
		banData:                 make(map[string]*banData),
		connections:             make(map[net.Conn]*playerData),
		verificationKeys:        make(map[string]net.Conn),
		denyIPMatch:             make(map[ipPair]time.Time),
		pools:                   make(map[int]map[uint32]*playerData),
		poolAmounts:             make(map[int]uint64),
		poolVoters:              make(map[int]int),
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
func (t *Tracker) bannedByPool(p *playerData, lock bool) bool {
	if lock {
		t.mutex.RLock()
		defer t.mutex.RUnlock()
	}

	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// the vote is all available voters - 1 for the accused
	return len(p.blamedBy) >= t.poolVoters[p.pool]-1
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

	banData := t.banData[ip]
	if banData != nil && banData.score >= maxBanScore {
		return true
	}

	return false
}

// addDenyIPMatch prevents an IP from joining a pool with the other
// pool member IPs for a timeout period.
func (t *Tracker) addDenyIPMatch(player1 net.Conn, pool int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	ip, _, _ := net.SplitHostPort(player1.RemoteAddr().String())
	for _, otherPlayer := range t.pools[pool] {
		otherIP, _, _ := net.SplitHostPort(otherPlayer.conn.RemoteAddr().String())
		if ip == otherIP {
			fmt.Printf("SKIPPING BAN FOR %#v to %#v\n", ip, otherIP)
			continue
		}

		// if a ban somehow already exists, extend it
		fmt.Printf("ADDING REAL BAN FOR %#v to %#v\n", ip, otherIP)
		t.denyIPMatch[newIPPair(ip, otherIP)] = time.Now()
	}
}

// deniedByIPMatch returns true if an IP should be denied access to a pool.
// Caller should hold the mutex.
func (t *Tracker) deniedByIPMatch(player net.Conn, pool int) bool {
	ip, _, _ := net.SplitHostPort(player.RemoteAddr().String())
	for _, otherPlayer := range t.pools[pool] {
		otherIP, _, _ := net.SplitHostPort(otherPlayer.conn.RemoteAddr().String())
		fmt.Printf("CHECKING IP FOR POOL ENTRY %#v to %#v\n", ip, otherIP)

		if _, ok := t.denyIPMatch[newIPPair(ip, otherIP)]; ok {
			fmt.Printf("DENYING IP FOR POOL ENTRY %#v to %#v\n", ip, otherIP)
			return true
		}
	}

	fmt.Printf("NO DENIAL FOUND \n")
	return false
}

// CleanupDeniedByIPMatch removes timed out denyIPMatch entries.
func (t *Tracker) CleanupDeniedByIPMatch() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	for pair, deniedTime := range t.denyIPMatch {
		if deniedTime.Add(denyIPTime).Before(time.Now()) {
			delete(t.denyIPMatch, pair)
		}
	}
}

// increaseBanScore increases the ban score for an IP on the server.
func (t *Tracker) increaseBanScore(conn net.Conn) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	if _, ok := t.banData[ip]; ok {
		t.banData[ip].score += banScoreTick
	} else {
		t.banData[ip] = &banData{
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

	if _, ok := t.banData[ip]; ok {
		t.banData[ip].score -= banScoreTick
	}

	if t.banData[ip].score == 0 {
		delete(t.banData, ip)
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

// playerCount returns the number of players in a pool.
func (t *Tracker) playerCount(pool int) int {
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
	num, playerNum, found := t.getAvailableSlot(p)
	if !found {
		num = t.getEmptyPool()
		playerNum = firstPlayerNum
		t.pools[num] = make(map[uint32]*playerData)
		t.poolAmounts[num] = p.amount
		t.poolVoters[num] = t.poolSize
		t.poolVersions[num] = p.version
		t.poolTypes[num] = p.shuffleType
	}

	t.pools[num][playerNum] = p

	if len(t.pools[num]) == t.poolSize {
		t.fullPools[num] = nil
	}

	return num, playerNum
}

// getAvailableSlot finds an existing pool and open player slot for player data
func (t *Tracker) getAvailableSlot(p *playerData) (int, uint32, bool) {
	for num := range t.pools {
		if t.poolAmounts[num] != p.amount {
			continue
		}

		if t.poolVersions[num] != p.version {
			continue
		}

		if t.poolTypes[num] != p.shuffleType {
			continue
		}

		if _, ok := t.fullPools[num]; ok {
			continue
		}

		if t.deniedByIPMatch(p.conn, num) {
			continue
		}

		// find a slot in the available pool
		playerNum := firstPlayerNum
		for {
			if _, ok := t.pools[num][playerNum]; ok {
				playerNum = playerNum + 1
				continue
			}
			break
		}

		return num, playerNum, true
	}
	return 0, 0, false
}

// getEmptyPool finds the lowest empty pool number >=1
func (t *Tracker) getEmptyPool() int {
	num := firstPoolNum
	for {
		if _, ok := t.pools[num]; !ok {
			return num
		}
		num++
	}
}

// decreasePoolVoters decreases the voter count for a pool.
func (t *Tracker) decreasePoolVoters(pool int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.poolVoters[pool]--
}

// unassignPool removes a user from a pool.
// This method assumes the caller is holding the mutex.
func (t *Tracker) unassignPool(p *playerData) {
	delete(t.pools[p.pool], p.number)

	if len(t.pools[p.pool]) == 0 {
		delete(t.pools, p.pool)
		delete(t.fullPools, p.pool)
		delete(t.poolAmounts, p.pool)
		delete(t.poolVoters, p.pool)
		delete(t.poolVersions, p.pool)
		delete(t.poolTypes, p.pool)
	}
}
