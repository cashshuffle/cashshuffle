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
	T       *testing.T
}

type testClient struct {
	h          *testHarness
	conn       net.Conn
	Inbox      *testInbox
	amount     uint64
	version    uint64
	session    []byte
	playerNum  uint32
}

// new TestHarness sets up the required parts for automating a shuffle
func testNewHarness(t *testing.T) *testHarness {
	// prepare shuffle environment: tracker, packet channel, connections
	anyPort := 0
	poolSize := 4
	tracker := NewTracker(poolSize, anyPort , anyPort , anyPort , anyPort)

	piChan := make(chan *packetInfo)
	go startPacketInfoChan(piChan)
	return &testHarness{
		tracker: tracker,
		packets: piChan,
		T:       t}
}

// NewClient creates a client with an in-memory connection to server
func (h *testHarness) NewClient() *testClient {
	clientConn, serverConn := net.Pipe()

	// handle the server side of the connection
	go handleConnection(serverConn, h.packets, h.tracker)

	// handle the client side of the connection
	inbox := testNewInbox(clientConn)

	// return a client that has done nothing but connect
	return &testClient {
		h:          h,
		conn:       clientConn,
		Inbox:      inbox,
	}
}

// Register has each connection request a registration
// https://github.com/cashshuffle/cashshuffle/wiki/CashShuffle-Server-Specification#entering-a-shuffle
func (tc *testClient) Register(amount, version uint64) {
	tc.amount = amount
	tc.version = version

	testTempKey++
	registrationMessage := &message.Signed{
		Packet: &message.Packet{
			FromKey: &message.VerificationKey{
				Key: strconv.Itoa(testTempKey),
			},
			Registration: &message.Registration{
				Amount: amount,
				Version: version,
				Type: message.ShuffleType_DEFAULT,
			},
		},
		Signature: nil,
	}
	err := writeMessage(tc.conn, []*message.Signed{registrationMessage})
	if err != nil {
		tc.h.T.Fatal(err)
	}

	// check the response
	//response := tc.Inbox.WaitOldest()
	//fmt.Println("Register: got response:", response)
	//tc.playerNum = response.message.GetPacket()[0].Packet.Number
	//tc.session = response.message.GetPacket()[0].Packet.Session
}

type testInbox struct {
	packets []*packetInfo
	mutex   sync.Mutex
}

func testNewInbox(conn net.Conn) *testInbox {
	packetChan := make(chan *packetInfo)
	packets := make([]*packetInfo, 0)
	m := sync.Mutex{}
	go func() {
		processMessages(conn, packetChan, nil)
		for pi := range packetChan {
			m.Lock()
			packets = append(packets, pi)
			m.Unlock()
			fmt.Println("received inbox message")
		}
	}()
	return &testInbox{
		packets: packets,
		mutex:   m,
	}
}

func (ib *testInbox) WaitOldest() *packetInfo {
	fmt.Println("WaitOldest: start waiting")
	for {
		if len(ib.packets) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	ib.mutex.Lock()
	packet := ib.packets[0]
	ib.packets = ib.packets[1:]
	ib.mutex.Unlock()
	return packet
}


func TestRegistration(t *testing.T) {
	h := testNewHarness(t)
	clients := []*testClient{
		h.NewClient(),
		//h.NewClient(),
		//h.NewClient(),
	}

	bch1 := uint64(100000000)
	version := uint64(999)
	for _, client := range clients {
		client.Register(bch1, version)
	}


	fmt.Println("inbox before: ", clients[0].Inbox.packets)
	time.Sleep(time.Second * 1)
	fmt.Println("inbox after: ", clients[0].Inbox.packets)

	assert.Equal(t, 3, len(h.tracker.connections))
	//assert.Equal(t, 1, len(h.Tracker.pools))
	//assert.Equal(t, map[int]uint64{1: bch1}, h.Tracker.poolAmounts)
	//assert.Equal(t, map[int]message.ShuffleType{1: message.ShuffleType_DEFAULT}, h.Tracker.poolTypes)
	//assert.Equal(t, map[int]uint64{1: 999}, h.Tracker.poolVersions)
}
