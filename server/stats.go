package server

// StatsInformer defines an interface that exposes tracker stats
type StatsInformer interface {
	Stats(string, bool) *TrackerStats
}

// TrackerStats represents a snapshot of the trackers statistics
type TrackerStats struct {
	BanScore             uint32      `json:"banScore"`
	Banned               bool        `json:"banned"`
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
	Type    string `json:"type"`
	Full    bool   `json:"full"`
	Version uint64 `json:"version"`
}

// Stats returns the tracker stats.
func (t *Tracker) Stats(ip string, tor bool) *TrackerStats {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	banned := false
	var banScore uint32

	banData := t.bannedIPs[ip]
	if banData != nil {
		banScore = banData.score
		if banData.score >= maxBanScore {
			banned = true
		}
	}

	sp := t.shufflePort
	if tor {
		sp = t.torShufflePort
	}

	wssp := t.shuffleWebSocketPort
	if tor {
		wssp = t.torShuffleWebSocketPort
	}

	ts := &TrackerStats{
		BanScore:             banScore,
		Banned:               banned,
		Connections:          len(t.connections),
		PoolSize:             t.poolSize,
		Pools:                make([]PoolStats, 0),
		ShufflePort:          sp,
		ShuffleWebSocketPort: wssp,
	}

	for k, p := range t.pools {
		_, full := t.fullPools[k]
		ps := PoolStats{
			Members: len(p),
			Amount:  t.poolAmounts[k],
			Type:    t.poolTypes[k].String(),
			Full:    full,
			Version: t.poolVersions[k],
		}
		ts.Pools = append(ts.Pools, ps)
	}

	return ts
}
