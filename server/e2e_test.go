package server

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cashshuffle/cashshuffle/message"
)

var (
	testTempKey int
)

// testHarness holds the pieces required for automating a shuffle
type testHarness struct {
	tracker *Tracker
	packets chan *packetInfo
	t       *testing.T
}

type testClient struct {
	h         *testHarness
	conn      net.Conn
	inbox     *testInbox
	amount    uint64
	version   uint64
	session   []byte
	playerNum uint32
}

// new TestHarness sets up the required parts for automating a shuffle
func testNewHarness(t *testing.T, poolSize int) *testHarness {
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

// NewClient creates a client with an in-memory connection to server
func (h *testHarness) NewClient() *testClient {
	clientConn, serverConn := net.Pipe()

	// handle the server side of the connection
	go handleConnection(serverConn, h.packets, h.tracker)

	// handle the client side of the connection
	inbox := testNewInbox(clientConn)

	// return a client that has done nothing but connect
	return &testClient{
		h:     h,
		conn:  clientConn,
		inbox: inbox,
	}
}

// Register has each connection request a registration
// https://github.com/cashshuffle/cashshuffle/wiki/CashShuffle-Server-Specification#entering-a-shuffle
func (client *testClient) Register(amount, version uint64) {
	client.amount = amount
	client.version = version

	testTempKey++
	registrationMessage := &message.Signed{
		Packet: &message.Packet{
			FromKey: &message.VerificationKey{
				Key: strconv.Itoa(testTempKey),
			},
			Registration: &message.Registration{
				Amount:  amount,
				Version: version,
				Type:    message.ShuffleType_DEFAULT,
			},
		},
		Signature: nil,
	}

	err := writeMessage(client.conn, []*message.Signed{registrationMessage})
	if err != nil {
		client.h.t.Fatal(err)
	}

	client.playerNum, client.session = client.h.AssertRegistered(client)
}

type testInbox struct {
	packets []*packetInfo
	mutex   sync.Mutex
}

func testNewInbox(conn net.Conn) *testInbox {
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

func (ib *testInbox) PopOldest() (*packetInfo, error) {
	var p *packetInfo

	for i := 0; i < 2; i++ {
		ib.mutex.Lock()
		if len(ib.packets) > 0 {
			p = ib.packets[0]
			ib.packets = ib.packets[1:]
			ib.mutex.Unlock()
			return p, nil
		}
		ib.mutex.Unlock()
		time.Sleep(1 * time.Millisecond)
	}
	return nil, fmt.Errorf("empty inbox")
}

type poolState struct {
	value   uint64
	version uint64
	players int
	isFull  bool
}

func TestHappyShuffle(t *testing.T) {
	poolSize := 3
	h := testNewHarness(t, poolSize)
	bch1 := uint64(100000000)
	version := uint64(999)

	// clients join the pool, one at a time
	clients := make([]*testClient, 0)
	for i := 0; i < poolSize; i++ {
		client := h.NewClient()
		clients = append(clients, client)

		client.Register(bch1, version)
		if i < poolSize-1 {
			h.AssertBroadcastNewPlayer(client, clients)
		}
	}
	// the pool is full and the shuffle starts
	h.AssertBroadcastPhase1Announcement(clients)

	// confirm basic state on the server side
	h.AssertPools([]poolState{
		{
			value:   bch1,
			version: version,
			players: len(clients),
			isFull:  true,
		},
	})

	// the shuffle succeeded, and clients leave with no blame
	for _, c := range clients {
		err := c.conn.Close()
		if err != nil {
			t.Fatal(err)
		}
	}

	// after the clients leave, the pool should be removed
	h.AssertPools([]poolState{})

	// all messages should be consumed through the assertions
	// if anything is remaining, it is outside of specification
	// or something unexpected happened
	h.AssertEmptyInboxes(clients)
}

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

func (h *testHarness) AssertBroadcastNewPlayer(c *testClient, all []*testClient) {
	for _, eachClient := range all {
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

func (h *testHarness) AssertPools(states []poolState) {
	// wait for the server to catch up
	for i := 0; i < 2; i++ {
		if len(h.tracker.pools) != len(states) {
			time.Sleep(1 * time.Millisecond)
			continue
		}
		break
	}

	// convert pools into simple states
	actualStates := make([]poolState, 0)
	for poolNum := range h.tracker.pools {
		_, isFull := h.tracker.fullPools[poolNum]
		actualStates = append(actualStates, poolState{
			value:   h.tracker.poolAmounts[poolNum],
			version: h.tracker.poolVersions[poolNum],
			players: len(h.tracker.pools[poolNum]),
			isFull:  isFull,
		})
	}

	// check for each state in the pools
	assert.ElementsMatch(h.t, states, actualStates)
}

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
