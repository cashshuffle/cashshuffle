package server

import (
	"net"
	"sync"

	"github.com/nats-io/nuid"
)

// tracker is used to track connections to the server.
type tracker struct {
	connections map[*net.Conn]*trackerData
	mutex       sync.Mutex
}

// trackerData is data needed about each connection.
type trackerData struct {
	mutex     sync.Mutex
	sessionID string
	number    int64
}

// createSessionID generates a unique session id.
func (td *trackerData) createSessionID() {
	td.mutex.Lock()
	defer td.mutex.Unlock()

	n := nuid.New()
	td.sessionID = n.Next()
}

// init initializes the tracker.
func (t *tracker) init() {
	t.connections = make(map[*net.Conn]*trackerData)

	return
}

// add adds a connection to the tracker.
func (t *tracker) add(conn *net.Conn, data *trackerData) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.connections[conn] = data

	return
}

// remove removes the connection.
func (t *tracker) remove(conn *net.Conn) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	delete(t.connections, conn)

	return
}

// count returns the number of connections to the server.
func (t *tracker) count() int {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return len(t.connections)
}
