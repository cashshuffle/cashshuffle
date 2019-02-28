package server

import (
	"net"
	"testing"
	"time"

	"github.com/cashshuffle/cashshuffle/message"
	"github.com/stretchr/testify/assert"
)

func TestTrackStats(t *testing.T) {
	tracker := &Tracker{
		connections: map[net.Conn]*trackerData{
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
			&fakeConn{}: {},
		},
		pools: map[int]map[uint32]interface{}{
			1: {
				1: nil,
				2: nil,
				3: nil,
				4: nil,
				5: nil,
			},
			2: {
				6: nil,
				7: nil,
				8: nil,
			},
		},
		poolAmounts: map[int]uint64{
			1: 100,
			2: 1000,
		},
		poolTypes: map[int]message.ShuffleType{
			1: 0,
			2: 1,
		},
		poolVersions: map[int]uint64{
			1: 0,
			2: 1,
		},
		fullPools: map[int]interface{}{
			1: nil,
		},
		poolSize:    5,
		shufflePort: 3000,
		bannedIPs: map[string]*banData{
			"8.8.8.8": &banData{
				score: maxBanScore,
			},
			"8.8.4.4": &banData{
				score: maxBanScore - 1,
			},
		},
	}

	// Test with ban.
	stats := tracker.Stats("8.8.8.8")

	assert.Equal(t, uint32(3), stats.BanScore)
	assert.Equal(t, true, stats.Banned)
	assert.Equal(t, 8, stats.Connections)
	assert.Equal(t, 5, stats.PoolSize)
	assert.Equal(t, 2, len(stats.Pools))
	assert.Equal(t, 3000, stats.ShufflePort)
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
	stats2 := tracker.Stats("8.8.4.4")

	assert.Equal(t, uint32(2), stats2.BanScore)
	assert.Equal(t, false, stats2.Banned)
	assert.Equal(t, 8, stats2.Connections)
	assert.Equal(t, 5, stats2.PoolSize)
	assert.Equal(t, 2, len(stats2.Pools))
	assert.Equal(t, 3000, stats2.ShufflePort)
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
