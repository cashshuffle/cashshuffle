package server

// StatsInformer defines an interface that exposes tracker stats
type StatsInformer interface {
	Stats() *TrackerStats
}

// TrackerStats represents a snapshot of the trackers statistics
type TrackerStats struct {
	Connections          int         `json:"connections"`
	PoolSize             int         `json:"poolSize"`
	Pools                []PoolStats `json:"pools"`
	ShufflePort          int         `json:"shufflePort"`
	ShuffleWebSocketPort int         `json:"shuffleWebSocketPort"`
}

// PoolStats represents the stats for a particular pool
type PoolStats struct {
	Members int    `json:"members"`
	Amount  uint64 `json:"amount"`
	Full    bool   `json:"full"`
}

// Stats returns the tracker stats.
func (t *Tracker) Stats() *TrackerStats {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	ts := &TrackerStats{
		Connections:          len(t.connections),
		PoolSize:             t.poolSize,
		Pools:                make([]PoolStats, 0),
		ShufflePort:          t.shufflePort,
		ShuffleWebSocketPort: t.shuffleWebSocketPort,
	}

	for k, p := range t.pools {
		_, full := t.fullPools[k]
		ps := PoolStats{
			Members: len(p),
			Amount:  t.poolAmounts[k],
			Full:    full,
		}
		ts.Pools = append(ts.Pools, ps)
	}

	return ts
}
