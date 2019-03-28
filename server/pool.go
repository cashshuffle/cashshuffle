package server

import (
	"sync"

	"github.com/cashshuffle/cashshuffle/message"
)

const (
	// firstPlayerNum is the starting number for players in a pool
	firstPlayerNum = uint32(1)
)

// Pool groups players together for a shuffle
type Pool struct {
	num            int
	mutex          sync.RWMutex
	players        map[uint32]*PlayerData
	size           int
	amount         uint64
	firstBan       *PlayerData
	version        uint64
	shuffleType    message.ShuffleType
	frozenSnapshot map[string]*PlayerData // vk > player
}

// newPool creates a new pool and enforces the rule that pools only exist
// with at least one player.
func newPool(num int, player *PlayerData, size int) *Pool {
	pool := &Pool{
		num:            num,
		size:           size,
		mutex:          sync.RWMutex{},
		players:        make(map[uint32]*PlayerData),
		amount:         player.amount,
		firstBan:       nil,
		version:        player.version,
		shuffleType:    player.shuffleType,
		frozenSnapshot: make(map[string]*PlayerData),
	}
	pool.AddPlayer(player)
	return pool
}

// IsFrozen returns true if the pool is frozen and will not add new players.
func (pool *Pool) IsFrozen() bool {
	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	return len(pool.frozenSnapshot) != 0
}

// IsBanned returns true if the player has been banned by their pool.
// This assumes that only one ban will happen per pool.
func (pool *Pool) IsBanned(player *PlayerData) bool {
	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	// the vote is all available voters - 1 for the accused
	return len(player.blamedBy) >= pool.size-1
}

// PlayerCount returns the number of players in a pool.
func (pool *Pool) PlayerCount() int {
	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	return len(pool.players)
}

// AddPlayer attempts to place player in the pool and returns success boolean
func (pool *Pool) AddPlayer(player *PlayerData) bool {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	if pool.amount != player.amount {
		return false
	}

	if pool.version != player.version {
		return false
	}

	if pool.shuffleType != player.shuffleType {
		return false
	}

	if len(pool.frozenSnapshot) != 0 {
		return false
	}

	// find a slot
	playerNum := firstPlayerNum
	for {
		if _, ok := pool.players[playerNum]; ok {
			playerNum = playerNum + 1
			continue
		}
		break
	}
	player.number = playerNum
	player.pool = pool
	pool.players[player.number] = player

	if len(pool.players) == pool.size {
		pool.frozenSnapshot = pool.takeSnapshot()
	}
	return true
}

// RemovePlayer removes a player from the pool
// This method depends on the caller discard the pool if empty.
func (pool *Pool) RemovePlayer(player *PlayerData) {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	delete(pool.players, player.number)
}

// PlayerFromSnapshot returns the user for the key or nil if they
// are not in the snapshot.
func (pool *Pool) PlayerFromSnapshot(verificationKey string) *PlayerData {
	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	if pool.frozenSnapshot == nil {
		return nil
	}
	return pool.frozenSnapshot[verificationKey]
}

// takeSnapshot gets a static lookup of all players in the pool
// This method assumes the caller is holding the mutex.
func (pool *Pool) takeSnapshot() map[string]*PlayerData {
	snapshot := make(map[string]*PlayerData)
	for _, p := range pool.players {
		snapshot[p.verificationKey] = p
	}
	return snapshot
}
