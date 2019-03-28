package server

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTrackStats(t *testing.T) {
	tracker := &Tracker{
		connections: map[net.Conn]*PlayerData{
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
		},
		denyIPMatch: map[ipPair]time.Time{},
		pools: map[int]*Pool{
			1: {
				num: 1,
				players: map[uint32]*PlayerData{
					1: nil,
					2: nil,
					3: nil,
					4: nil,
					5: nil,
				},
				size:        5,
				amount:      100,
				shuffleType: 0,
				version:     0,
				frozenSnapshot: map[string]*PlayerData{
					"1": nil,
					"2": nil,
					"3": nil,
					"4": nil,
					"5": nil,
				},
			},
			2: {
				num: 2,
				players: map[uint32]*PlayerData{
					6: nil,
					7: nil,
					8: nil,
				},
				size:           5,
				amount:         1000,
				shuffleType:    1,
				version:        1,
				frozenSnapshot: map[string]*PlayerData{},
			},
		},
		poolSize:                5,
		shufflePort:             3000,
		shuffleWebSocketPort:    3001,
		torShufflePort:          3002,
		torShuffleWebSocketPort: 3003,
		banData: map[string]*banData{
			"8.8.8.8": {
				score: maxBanScore,
			},
			"8.8.4.4": {
				score: maxBanScore - 1,
			},
		},
	}

	// Test with ban.
	stats := tracker.Stats("8.8.8.8", false)

	assert.Equal(t, uint32(5), stats.BanScore)
	assert.Equal(t, true, stats.Banned)
	assert.Equal(t, 8, stats.Connections)
	assert.Equal(t, 5, stats.PoolSize)
	assert.Equal(t, 2, len(stats.Pools))
	assert.Equal(t, 3000, stats.ShufflePort)
	assert.Equal(t, 3001, stats.ShuffleWebSocketPort)
	assert.Contains(t, stats.Pools,
		PoolStats{
			Members: 5,
			Amount:  100,
			Type:    "DEFAULT",
			Full:    true,
			Version: 0,
		},
		PoolStats{
			Members: 3,
			Amount:  1000,
			Type:    "DUST",
			Full:    false,
			Version: 1,
		},
	)

	// Test without ban.
	stats2 := tracker.Stats("8.8.4.4", true)

	assert.Equal(t, uint32(4), stats2.BanScore)
	assert.Equal(t, false, stats2.Banned)
	assert.Equal(t, 8, stats2.Connections)
	assert.Equal(t, 5, stats2.PoolSize)
	assert.Equal(t, 2, len(stats2.Pools))
	assert.Equal(t, 3002, stats2.ShufflePort)
	assert.Equal(t, 3003, stats2.ShuffleWebSocketPort)
	assert.Contains(t, stats2.Pools,
		PoolStats{
			Members: 5,
			Amount:  100,
			Type:    "DEFAULT",
			Full:    true,
			Version: 0,
		},
		PoolStats{
			Members: 3,
			Amount:  1000,
			Type:    "DUST",
			Full:    false,
			Version: 1,
		},
	)
}

type fakeConn struct {
	f interface{}
}

func (fc *fakeConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (fc *fakeConn) Write(b []byte) (n int, err error)  { return 0, nil }
func (fc *fakeConn) Close() error                       { return nil }
func (fc *fakeConn) LocalAddr() net.Addr                { return &fakeAddr{} }
func (fc *fakeConn) RemoteAddr() net.Addr               { return &fakeAddr{} }
func (fc *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (fc *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (fc *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

type fakeAddr struct{}

func (fa *fakeAddr) Network() string { return "" }
func (fa *fakeAddr) String() string  { return "" }
