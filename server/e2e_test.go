package server

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cashshuffle/cashshuffle/message"

	"github.com/avast/retry-go"
	"github.com/stretchr/testify/assert"

	log "github.com/sirupsen/logrus"
)

const (
	basicPoolSize = 3
	testAmount    = uint64(100000000)
	testVersion   = uint64(999)
)

var (
	testTempKey int
)

// TestHappyShuffle simulates a complete shuffle.
func TestHappyShuffle(t *testing.T) {
	h := newTestHarness(t, basicPoolSize)
	clients := h.NewPool(basicPoolSize, testAmount, testVersion, nil)

	// the announcement phase is reached so everyone must make at least one
	// broadcast to avoid "passive" label and ban score
	for _, c := range clients {
		c.BroadcastVerificationKey(clients)
	}

	// the shuffle succeeded, and clients leave with no blame
	for _, c := range clients {
		c.Disconnect()
	}

	// after the clients leave, the pool should be removed
	h.AssertPoolStates([]testPoolState{}, true)

	// everyone should have a clean record with no bans
	noBans := make([]testServerBanData, 0)
	h.AssertServerBans(noBans)

	// confirm no out of spec messages
	h.WaitEmptyInboxes(clients)
}

// TestUnanimousBlamesLeadToServerBan confirms an increase in ban score
// when a player is blamed by all other players in their pool, with repeated
// unanimous blames leading to a server ban.
func TestUnanimousBlamesLeadToServerBan(t *testing.T) {
	poolSize := 5
	h := newTestHarness(t, poolSize)
	troubleClient := newTestClient(h)
	expectedBanData := make([]testServerBanData, 0)

	// repeat the new pool and blame process until troubleClient is banned
	for i := 0; i < maxBanScore; i++ {
		// make a new pool
		otherClients := h.NewPool(poolSize, testAmount, testVersion, troubleClient)
		allClients := append([]*testClient{troubleClient}, otherClients...)
		// everyone say something to avoid passive ban
		for _, c := range allClients {
			c.BroadcastVerificationKey(allClients)
		}

		// all but one other client blames troubleClient
		otherClients[0].Blame(troubleClient, allClients)

		// troubleClient tries to avoid blame by disconnecting!
		// It should not work - troubleClient should still be banned eventually.
		troubleClient.Disconnect()

		// Blames continue.
		// Also since troubleClient disconnected, we only expect notifications
		// to go to remaining clients.
		otherClients[1].Blame(troubleClient, otherClients)

		// duplicate blames from the same client should not be counted twice!
		otherClients[2].Blame(troubleClient, otherClients)
		otherClients[2].Blame(troubleClient, otherClients)

		// ban score does not change yet because it is not a unanimous vote
		h.AssertServerBans(expectedBanData)
		// the last other client also blames troubleClient
		// this should cause troubleClient to be banned from the current pool
		otherClients[3].Blame(troubleClient, otherClients)
		// troubleClient also gets a new or increased ban score
		// because this is now a unanimous vote
		if len(expectedBanData) == 0 {
			// create new ban score
			expectedBanData = append(expectedBanData, testServerBanData{
				client:  troubleClient,
				banData: banData{score: 1},
			})
		} else {
			// increase existing ban score
			expectedBanData[0].banData.score++
		}
		h.AssertServerBans(expectedBanData)

		// remaining players try to blame one of themselves, but the pool
		// only allows one ban even with a full vote.
		for _, c := range otherClients {
			c.Blame(otherClients[0], otherClients)
		}
		h.AssertServerBans(expectedBanData)

		// everybody drops
		for _, c := range allClients {
			c.Disconnect()
		}
		// confirm no out of spec messages
		h.WaitEmptyInboxes(allClients)
	}

	// confirm that troubleClient is banned and cannot connect to the server
	troubleClient.Connect()
	h.WaitNotConnected(troubleClient)

	// confirm after time limit troubleClient can connect again
	// This could be done with minimal server changes by making ban
	// cleanup time a variable, setting it very short for this test
	// and then waiting for it to clean up.
}

// TestBlameOnlyWithinPool confirms specifically that players in different
// pools cannot blame each other and more generally that the server
// does not broadcast messages between pools.
func TestBlameAndBroadcastOnlyWithinPool(t *testing.T) {
	h := newTestHarness(t, basicPoolSize)
	poolA := h.NewPool(basicPoolSize, testAmount, testVersion, nil)
	poolB := h.NewPool(basicPoolSize, testAmount, testVersion, nil)

	// All of poolA blames one of poolB users
	// but no notifications should be generated
	noNotifications := make([]*testClient, 0)
	for _, cA := range poolA {
		// Note: Sending the invalid blame causes client to be forgotten
		cA.Blame(poolB[0], noNotifications)
	}
	// and no ban scores should appear
	noBanData := make([]testServerBanData, 0)
	h.AssertServerBans(noBanData)

	// confirm no out of spec messages
	h.WaitEmptyInboxes(poolA)
	h.WaitEmptyInboxes(poolB)
}

// TestOnlyUniqueVerificationKeysCanRegister confirms that a player with
// a non-unique verification key is not allowed to register.
func TestOnlyUniqueVerificationKeysCanRegister(t *testing.T) {
	var expectSuccess bool
	h := newTestHarness(t, basicPoolSize)

	// client registers normally
	client := newTestClient(h)
	client.Connect()
	expectSuccess = true
	client.Register(testAmount, testVersion, []*testClient{client}, false, expectSuccess)

	// client with same verification key is unable to register
	clone := &testClient{
		h:               client.h,
		verificationKey: client.verificationKey,
	}
	clone.Connect()
	expectSuccess = false
	clone.Register(testAmount, testVersion, nil, false, expectSuccess)
}

// TestPassivePlayersGetBanEffects confirms that a silent player who never
// announces their verification key after ANNOUNCEMENT phase gets a server
// ban score and also gets IP banned from the other players.
func TestPassivePlayersGetBanEffects(t *testing.T) {
	h := newTestHarness(t, basicPoolSize)
	passiveClient := newTestClient(h)
	otherClients := h.NewPool(basicPoolSize, testAmount, testVersion, passiveClient)
	allClients := append([]*testClient{passiveClient}, otherClients...)

	// The announcement phase is reached so everyone must make at least one
	// broadcast to avoid "passive" label and ban score.
	// However one client is passive and does nothing.
	for _, c := range otherClients {
		c.BroadcastVerificationKey(allClients)
	}

	// The shuffle fails, and clients leave. They are not able to blame
	// the passive user since they do not have the verification key.
	for _, c := range allClients {
		c.Disconnect()
	}

	// however the server noticed the passive user and gives them a global ban
	// score as well as p2p ban for the other players in the pool.
	passiveGotBanScore := []testServerBanData{
		{
			client:  passiveClient,
			banData: banData{score: 1},
		},
	}
	h.AssertServerBans(passiveGotBanScore)

	// confirm no out of spec messages
	h.WaitEmptyInboxes(allClients)
}

// testHarness holds the pieces required for automating a shuffle.
type testHarness struct {
	tracker *Tracker
	packets chan *packetInfo
	t       *testing.T
}

// newTestHarness sets up the required parts for automating a shuffle.
func newTestHarness(t *testing.T, poolSize int) *testHarness {
	log.SetLevel(log.DebugLevel)
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

// NewPool simulates one pool filling up and consumes all expected messages.
// fixedClient will be used in place of one new client if provided.
// Returns all newly created clients.
func (h *testHarness) NewPool(poolSize int, value, version uint64,
	fixedClient *testClient) []*testClient {
	var c *testClient
	allClients := make([]*testClient, 0)
	newClients := make([]*testClient, 0)

	if fixedClient != nil {
		fixedClient.Disconnect()
	}

	// clients connect and join the pool, one at a time
	for i := 0; i < poolSize; i++ {
		if (fixedClient != nil) && (i == 0) {
			c = fixedClient
		} else {
			c = newTestClient(h)
			newClients = append(newClients, c)
		}
		allClients = append(allClients, c)

		c.Connect()
		isFullPool := i == poolSize-1
		c.Register(value, version, allClients, isFullPool, true)
	}
	return newClients
}

// testClient represents a single actor connected to the server.
type testClient struct {
	// setup on creation
	h               *testHarness
	verificationKey string
	// setup on connection
	conn       net.Conn
	remoteConn net.Conn
	inbox      *testInbox
	// setup on registration
	amount    uint64
	version   uint64
	session   []byte
	playerNum uint32
}

// newTestClient creates a client that is ready to connect to the server.
func newTestClient(h *testHarness) *testClient {
	testTempKey++
	return &testClient{
		h:               h,
		verificationKey: strconv.Itoa(testTempKey),
	}
}

// Connect sets up a connection between client and server.
func (c *testClient) Connect() {
	c.conn, c.remoteConn = net.Pipe()

	// handle the client side of the connection
	c.inbox = newTestInbox(c.conn)

	// handle the server side of the connection
	go handleConnection(c.remoteConn, c.h.packets, c.h.tracker)
}

// Disconnect simulates the client dropping the connection and confirms that
// the server is aware of the disconnect.
func (c *testClient) Disconnect() {
	// client side
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.h.t.Fatal(err)
		}
	}
	c.h.WaitNotConnected(c)
}

// Register sends a registration message to the server and consumes the
// registration response to complete the process.
// https://github.com/cashshuffle/cashshuffle/wiki/CashShuffle-Server-Specification#entering-a-shuffle
func (c *testClient) Register(amount, version uint64,
	shouldBeNotified []*testClient, isFullPool bool, expectSuccess bool) {
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

	if !expectSuccess {
		c.h.WaitNotConnected(c)
		return
	}

	// consume the registration response
	c.playerNum, c.session = c.h.WaitRegistered(c)

	if isFullPool {
		// consume the new round broadcast
		c.h.WaitBroadcastPhase1Announcement(shouldBeNotified)
	} else {
		// consume the new player broadcast
		c.h.WaitBroadcastNewPlayer(c, shouldBeNotified)
	}
	expectedStates := []testPoolState{
		{
			value:   c.amount,
			version: c.version,
			players: len(shouldBeNotified),
			isFull:  isFullPool,
		},
	}
	c.h.AssertPoolStates(expectedStates, false)
}

// BroadcastVerificationKey sends a minimal valid message that
// includes their verification key.
// https://github.com/cashshuffle/cashshuffle/wiki/CashShuffle-Server-Specification#player-messaging
func (c *testClient) BroadcastVerificationKey(shouldBeNotified []*testClient) {
	msg := &message.Signed{
		Packet: &message.Packet{
			Number:  c.playerNum,
			Session: c.session,
			FromKey: &message.VerificationKey{
				Key: c.verificationKey,
			},
		},
		Signature: nil,
	}

	err := writeMessage(c.conn, []*message.Signed{msg})
	if err != nil {
		c.h.t.Fatal(err)
	}
	c.h.WaitBroadcastVerificationKey(c.verificationKey, shouldBeNotified)
}

// Blame sends a blame message to the server against accused and asserts
// receipt of the blame by the list of clients.
// https://github.com/cashshuffle/cashshuffle/wiki/CashShuffle-Server-Specification#blame-messages
func (c *testClient) Blame(accused *testClient, shouldBeNotified []*testClient) {
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
						Key: accused.verificationKey,
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
	c.h.WaitBroadcastBlame(msg, shouldBeNotified)
}

// testInbox collects all incoming client messages in the background.
type testInbox struct {
	packets []*packetInfo
	mutex   sync.Mutex
}

// newTestInbox creates an active inbox watching the connection.
func newTestInbox(conn net.Conn) *testInbox {
	inbox := &testInbox{
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
			inbox.mutex.Lock()
			inbox.packets = append(inbox.packets, pi)
			inbox.mutex.Unlock()
		}
	}()
	return inbox
}

// PopOldest pops the oldest message and returns it or returns an error
// if no message appears within a short time period.
func (inbox *testInbox) PopOldest() (*packetInfo, error) {
	var packet *packetInfo

	err := retry.Do(
		func() error {
			inbox.mutex.Lock()
			defer inbox.mutex.Unlock()
			if len(inbox.packets) == 0 {
				return fmt.Errorf("empty inbox")
			}
			packet = inbox.packets[0]
			inbox.packets = inbox.packets[1:]
			return nil
		},
		retry.Attempts(100),
		retry.Delay(5*time.Millisecond),
		retry.DelayType(retry.FixedDelay),
		retry.LastErrorOnly(true),
	)
	return packet, err
}

// WaitRegistered waits for and consumes the registration response
// and returns registration details.
func (h *testHarness) WaitRegistered(c *testClient) (uint32, []byte) {
	// popping the message is where the wait happens
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

// WaitBroadcastNewPlayer confirms the client was broadcast to the pool
// and consumes all expected broadcast messages.
func (h *testHarness) WaitBroadcastNewPlayer(c *testClient, pool []*testClient) {
	for _, eachClient := range pool {
		// popping messages is where the wait happens
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

// WaitBroadcastPhase1Announcement confirms and consumes a phase 1 broadcast
// to all pool players.
func (h *testHarness) WaitBroadcastPhase1Announcement(all []*testClient) {
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

// WaitBroadcastVerificationKey confirms that a client's message was broadcast
// to the pool and consumes all expected broadcast messages.
func (h *testHarness) WaitBroadcastVerificationKey(vk string, pool []*testClient) {
	for _, eachClient := range pool {
		// popping messages is where the wait happens
		response, err := eachClient.inbox.PopOldest()
		if err != nil {
			h.t.Fatal(err)
		}
		signedPackets := response.message.GetPacket()
		assert.Len(h.t, signedPackets, 1)
		packet := signedPackets[0].Packet

		assert.Equal(h.t, vk, packet.GetFromKey().GetKey())
	}
}

// WaitBroadcastBlame confirms and consumes the blame broadcast to all
// pool players.
func (h *testHarness) WaitBroadcastBlame(expected *message.Signed, pool []*testClient) {
	for _, c := range pool {
		actualPacket, err := c.inbox.PopOldest()
		if err != nil {
			h.t.Fatal(err)
		}
		assert.Len(h.t, actualPacket.message.GetPacket(), 1)
		actualMsg := actualPacket.message.GetPacket()[0]
		assert.Equal(h.t, expected, actualMsg)
	}
}

// WaitNotConnected confirms that client's connection is not
// in the server's lookup table and that the connection has been closed.
func (h *testHarness) WaitNotConnected(c *testClient) {
	err := retry.Do(
		func() error {
			// first check the connection
			if c.conn != nil {
				_ = c.conn.SetReadDeadline(time.Now())
				_, err := c.conn.Read([]byte{})
				if (err != io.EOF) && (err != io.ErrClosedPipe) {
					return fmt.Errorf("connection still active (with retry error %v)", err)
				}
			}

			// next make sure the server drops the client from the tracker
			h.tracker.mutex.RLock()
			defer h.tracker.mutex.RUnlock()

			_, isTracked := h.tracker.connections[c.remoteConn]
			if isTracked {
				return errors.New("server thinks client is still connected")
			}
			return nil
		},
		retry.Attempts(100),
		retry.Delay(5*time.Millisecond),
		retry.DelayType(retry.FixedDelay),
		retry.LastErrorOnly(true),
	)
	if err != nil {
		h.t.Fatal(err)
	}
}

// testPoolState describes the state of a pool.
type testPoolState struct {
	value   uint64
	version uint64
	players int
	isFull  bool
}

// AssertPoolStates confirms the server pools are in the expected state.
// This must be called after finalization of any disconnects and
// after broadcast of any new players.
func (h *testHarness) AssertPoolStates(expected []testPoolState, completeState bool) {
	h.tracker.mutex.RLock()
	defer h.tracker.mutex.RUnlock()

	// convert pools into simple states
	actualStates := make([]testPoolState, 0)
	for _, pool := range h.tracker.pools {
		actualStates = append(actualStates, testPoolState{
			value:   pool.amount,
			version: pool.version,
			players: pool.PlayerCount(),
			isFull:  pool.IsFrozen(),
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

// testP2PBan identifies the two clients involved in a p2p ban.
type testP2PBan struct {
	a *testClient
	b *testClient
}

// AssertP2PBans confirms that the provided p2p bans exist on the server.
// This must be called after ban messages have been received.
func (h *testHarness) AssertP2PBans(bans []testP2PBan) {
	expectedPairs := make([]ipPair, 0)
	for _, ban := range bans {
		ipA := getIP(ban.a.remoteConn)
		ipB := getIP(ban.b.remoteConn)
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

// testServerBanData describes a client and their server ban status.
type testServerBanData struct {
	client  *testClient
	banData banData
}

// AssertServerBans confirms the status of server bans.
// This must be called after ban messages have been received.
func (h *testHarness) AssertServerBans(cbs []testServerBanData) {
	expectedBanData := make(map[string]banData)
	for _, bd := range cbs {
		ip := getIP(bd.client.remoteConn)
		expectedBanData[ip] = bd.banData
	}

	h.tracker.mutex.RLock()
	actualBanData := make(map[string]banData)
	for ip, bd := range h.tracker.banData {
		actualBanData[ip] = *bd
	}
	h.tracker.mutex.RUnlock()

	assert.Equal(h.t, expectedBanData, actualBanData)
}

// WaitEmptyInboxes confirms that clients received no unexpected messages.
func (h *testHarness) WaitEmptyInboxes(clients []*testClient) {
	err := retry.Do(
		func() error {
			lines := make([]string, 0)
			for _, c := range clients {
				c.inbox.mutex.Lock()
				if len(c.inbox.packets) != 0 {
					lines = append(lines, fmt.Sprintf("inbox #%d not empty:", c.playerNum))
					for _, pkt := range c.inbox.packets {
						lines = append(lines, fmt.Sprintf("  %+v", pkt.message.GetPacket()))
					}
				}
				c.inbox.mutex.Unlock()
			}
			if len(lines) != 0 {
				return errors.New(strings.Join(lines, "\n"))
			}
			return nil
		},
		retry.Attempts(100),
		retry.Delay(5*time.Millisecond),
		retry.DelayType(retry.FixedDelay),
		retry.LastErrorOnly(true),
	)
	if err != nil {
		h.t.Fatal(err)
	}
}
