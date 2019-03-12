package server

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cashshuffle/cashshuffle/message"

	"github.com/stretchr/testify/assert"
)

const (
	basicPoolSize = 3
	someAmount    = uint64(100000000)
	someVersion   = uint64(999)
)

var (
	testTempKey int
)

// TestHappyShuffle simulates a complete shuffle
func TestHappyShuffle(t *testing.T) {
	h := newTestHarness(t, basicPoolSize)
	clients := h.FillOnePool(basicPoolSize, someAmount, someVersion)

	// the shuffle succeeded, and clients leave with no blame
	for _, c := range clients {
		err := c.conn.Close()
		if err != nil {
			t.Fatal(err)
		}
	}

	// after the clients leave, the pool should be removed
	h.AssertPoolStates([]testPoolState{}, true)

	// all messages should be consumed through the assertions
	// if anything is remaining, it is outside of specification
	// or something unexpected happened
	h.AssertEmptyInboxes(clients)
}

// TestUnanimousBlamesLeadToServerBan confirms an increase in ban score
// when a player is blamed by all other players in their pool, with repeated
// unanimous blames leading to a server ban
func TestUnanimousBlamesLeadToServerBan(t *testing.T) {
	h := newTestHarness(t, 5)
	clients := h.FillOnePool(5, someAmount, someVersion)
	troubleClient := clients[0]

	// all but one other client blames troubleClient
	clients[1].Blame(troubleClient)
	clients[2].Blame(troubleClient)
	clients[3].Blame(troubleClient)
	// troubleClient does not get a ban score since it was not a unanimous vote
	h.AssertServerBans([]testServerBanData{})

	// the last other client also blames troubleClient
	clients[4].Blame(troubleClient)
	// troubleClient's ban score increases because this is now a unanimous vote
	h.AssertServerBans([]testServerBanData{
		{
			client: troubleClient,
			banData: banData{score: 1},
		},
	})
}

// testHarness holds the pieces required for automating a shuffle
type testHarness struct {
	tracker *Tracker
	packets chan *packetInfo
	t       *testing.T
}

// newTestHarness sets up the required parts for automating a shuffle
func newTestHarness(t *testing.T, poolSize int) *testHarness {
	// prepare shuffle environment: tracker, packet channel, connections
	anyPort := 0
	tracker := NewTracker(poolSize, anyPort, anyPort, anyPort, anyPort)
	piChan := make(chan *packetInfo)
	go startPacketInfoChan(piChan)

	return &testHarness{
		tracker: tracker,
		packets: piChan,
		t:       t,
	}
}

// FillOnePool simulates one pool filling up and consumes all expected messages.
func (h *testHarness) FillOnePool(poolSize int, value, version uint64) []*testClient {
	// clients join the pool, one at a time
	clients := make([]*testClient, 0)
	for i := 0; i < poolSize; i++ {
		client := h.NewTestClient()
		clients = append(clients, client)

		client.Register(value, version)
		if i < poolSize-1 {
			// new players are announced while the pool is not full
			h.AssertBroadcastNewPlayer(client, clients)
			h.AssertPoolStates([]testPoolState{
				{
					value:   value,
					version: version,
					players: len(clients),
					isFull:  false,
				},
			}, false)
		}
	}

	// the pool is full and the shuffle starts
	h.AssertBroadcastPhase1Announcement(clients)
	h.AssertPoolStates([]testPoolState{
		{
			value:   value,
			version: version,
			players: len(clients),
			isFull:  true,
		},
	}, false)
	return clients
}

// NewTestClient creates a client with an in-memory connection to server
func (h *testHarness) NewTestClient() *testClient {
	clientConn, serverConn := net.Pipe()

	// handle the server side of the connection
	go handleConnection(serverConn, h.packets, h.tracker)

	// handle the client side of the connection
	inbox := newTestInbox(clientConn)

	// return a client that has done nothing but connect
	testTempKey++
	return &testClient{
		h:               h,
		conn:            clientConn,
		remoteConn:      serverConn,
		verificationKey: strconv.Itoa(testTempKey),
		inbox:           inbox,
	}
}

// testClient represents a single actor connected to the server
type testClient struct {
	h               *testHarness
	conn            net.Conn
	remoteConn      net.Conn
	inbox           *testInbox
	amount          uint64
	version         uint64
	session         []byte
	verificationKey string
	playerNum       uint32
}

// Register sends a registration message to the server and completes the process
// https://github.com/cashshuffle/cashshuffle/wiki/CashShuffle-Server-Specification#entering-a-shuffle
func (c *testClient) Register(amount, version uint64) {
	c.amount = amount
	c.version = version

	msg := &message.Signed{
		Packet: &message.Packet{
			FromKey: &message.VerificationKey{
				Key: c.verificationKey,
			},
			Registration: &message.Registration{
				Amount:  amount,
				Version: version,
				Type:    message.ShuffleType_DEFAULT,
			},
		},
		Signature: nil,
	}

	err := writeMessage(c.conn, []*message.Signed{msg})
	if err != nil {
		c.h.t.Fatal(err)
	}

	c.playerNum, c.session = c.h.AssertRegistered(c)
}

// Blame sends a blame message to the server against other
// https://github.com/cashshuffle/cashshuffle/wiki/CashShuffle-Server-Specification#blame-messages
func (c *testClient) Blame(other *testClient) {
	msg := &message.Signed{
		Packet: &message.Packet{
			Number:  c.playerNum,
			Session: c.session,
			FromKey: &message.VerificationKey{
				Key: c.verificationKey,
			},
			Message: &message.Message{
				Blame: &message.Blame{
					Reason: message.Reason_LIAR,
					Accused: &message.VerificationKey{
						Key: other.verificationKey,
					},
				},
			},
		},
		Signature: nil,
	}

	err := writeMessage(c.conn, []*message.Signed{msg})
	if err != nil {
		c.h.t.Fatal(err)
	}
}

// testInbox collects all incoming client messages in the background
type testInbox struct {
	packets []*packetInfo
	mutex   sync.Mutex
}

// newTestInbox creates an active inbox watching the connection
func newTestInbox(conn net.Conn) *testInbox {
	ti := &testInbox{
		packets: make([]*packetInfo, 0),
		mutex:   sync.Mutex{},
	}
	packetChan := make(chan *packetInfo)

	// a client-side tracker is needed just for its channel closing machinery
	anyValue := 10
	placeholderTracker := NewTracker(anyValue, anyValue, anyValue, anyValue, anyValue)
	go processMessages(conn, packetChan, placeholderTracker)
	go func() {
		for pi := range packetChan {
			ti.mutex.Lock()
			ti.packets = append(ti.packets, pi)
			ti.mutex.Unlock()
		}
	}()
	return ti
}

// PopOldest pops the oldest message and returns it or returns an error
// if no message appears within a short time period.
func (ib *testInbox) PopOldest() (*packetInfo, error) {
	// just wait
	time.Sleep(2 * time.Millisecond)
	ib.mutex.Lock()
	defer ib.mutex.Unlock()

	if len(ib.packets) == 0 {
		return nil, fmt.Errorf("empty inbox")
	}

	p := ib.packets[0]
	ib.packets = ib.packets[1:]
	return p, nil
}

// AssertRegistered confirms registration, consumes expected registration
// message, and returns registration details
func (h *testHarness) AssertRegistered(c *testClient) (uint32, []byte) {
	response, err := c.inbox.PopOldest()
	if err != nil {
		h.t.Fatal(err)
	}
	signedPackets := response.message.GetPacket()
	assert.Len(h.t, signedPackets, 1)
	packet := signedPackets[0].Packet

	playerNum := packet.GetNumber()
	assert.NotEqual(h.t, 0, playerNum)

	session := packet.GetSession()
	assert.NotEqual(h.t, 0, len(session))

	return playerNum, session
}

// AssertBroadcastNewPlayer confirms the client was broadcast to the pool
// and consumes all expected broadcast messages
func (h *testHarness) AssertBroadcastNewPlayer(c *testClient, pool []*testClient) {
	for _, eachClient := range pool {
		response, err := eachClient.inbox.PopOldest()
		if err != nil {
			h.t.Fatal(err)
		}
		signedPackets := response.message.GetPacket()
		assert.Len(h.t, signedPackets, 1)
		packet := signedPackets[0].Packet

		assert.Equal(h.t, c.playerNum, packet.GetNumber())
	}
}

// AssertBroadcastPhase1Announcement confirms phase 1 was broadcast to the pool
// and consumes all expected broadcast messages
func (h *testHarness) AssertBroadcastPhase1Announcement(all []*testClient) {
	for _, eachClient := range all {
		response, err := eachClient.inbox.PopOldest()
		if err != nil {
			h.t.Fatal(err)
		}
		signedPackets := response.message.GetPacket()
		assert.Len(h.t, signedPackets, 1)
		packet := signedPackets[0].Packet

		assert.Equal(h.t, message.Phase_ANNOUNCEMENT, packet.GetPhase())
		assert.Equal(h.t, uint32(h.tracker.poolSize), packet.GetNumber())
	}
}

// testPoolState describes the state of a pool
type testPoolState struct {
	value   uint64
	version uint64
	players int
	isFull  bool
}

// AssertPoolStates confirms the server is in the expected state
func (h *testHarness) AssertPoolStates(expected []testPoolState, completeState bool) {
	// just wait
	time.Sleep(2 * time.Millisecond)
	h.tracker.mutex.RLock()
	defer h.tracker.mutex.RUnlock()

	h.tracker.mutex.RLock()
	defer h.tracker.mutex.RUnlock()
	// convert pools into simple states
	actualStates := make([]testPoolState, 0)
	for poolNum := range h.tracker.pools {
		_, isFull := h.tracker.fullPools[poolNum]
		actualStates = append(actualStates, testPoolState{
			value:   h.tracker.poolAmounts[poolNum],
			version: h.tracker.poolVersions[poolNum],
			players: len(h.tracker.pools[poolNum]),
			isFull:  isFull,
		})
	}

	if completeState {
		assert.ElementsMatch(h.t, actualStates, expected)
	} else {
		for _, s := range expected {
			assert.Contains(h.t, actualStates, s)
		}
	}
}

type testP2PBan struct {
	a *testClient
	b *testClient
}

func (h *testHarness) AssertP2PBans(bans []testP2PBan) {
	// just wait a short time
	time.Sleep(10 * time.Millisecond)

	expectedPairs := make([]ipPair, 0)
	for _, ban := range bans {
		ipA, _, _ := net.SplitHostPort(ban.a.remoteConn.RemoteAddr().String())
		ipB, _, _ := net.SplitHostPort(ban.b.remoteConn.RemoteAddr().String())
		expectedPairs = append(expectedPairs, newIPPair(ipA, ipB))
	}

	h.tracker.mutex.RLock()
	actualPairs := make([]ipPair, 0)
	for p := range h.tracker.denyIPMatch {
		actualPairs = append(actualPairs, p)
	}
	h.tracker.mutex.RUnlock()

	assert.ElementsMatch(h.t, expectedPairs, actualPairs)
}

type testServerBanData struct {
	client  *testClient
	banData banData
}

func (h *testHarness) AssertServerBans(cbs []testServerBanData) {
	// just wait a short time
	time.Sleep(10 * time.Millisecond)

	// convert client bans into server-side bans
	expectedBanData := make(map[string]banData)
	for _, bd := range cbs {
		ip, _, _ := net.SplitHostPort(bd.client.remoteConn.RemoteAddr().String())
		expectedBanData[ip] = bd.banData
	}

	h.tracker.mutex.RLock()
	defer h.tracker.mutex.RUnlock()
	actualBanData := make(map[string]banData)
	for ip, bd := range h.tracker.banData {
		actualBanData[ip] = *bd
	}

	assert.Equal(h.t, expectedBanData, actualBanData)
}

// AssertEmptyInboxes confirms that clients received no unexpected messages
func (h *testHarness) AssertEmptyInboxes(clients []*testClient) {
	for _, c := range clients {
		c.inbox.mutex.Lock()
		if len(c.inbox.packets) != 0 {
			e := fmt.Sprintf("inbox #%d not empty:\n", c.playerNum)
			for _, pkt := range c.inbox.packets {
				e = e + fmt.Sprintf("  %+v\n", pkt.message.GetPacket())
			}
			h.t.Fatalf(e)
		}
	}
}
