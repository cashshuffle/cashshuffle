package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/websocket"
)

var debugMode bool

// Start brings up the TCP server.
func Start(ip string, port int, cert string, key string, debug bool, t *Tracker, m *autocert.Manager) (err error) {
	var listener net.Listener

	debugMode = debug

	if tlsEnabled(cert, key, m) {
		listener, err = createTLSListener(ip, port, cert, key, m)
		if err != nil {
			return err
		}
	} else {
		listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port))
		if err != nil {
			return err
		}
	}

	defer listener.Close()

	packetInfoChan := make(chan *packetInfo)
	go startPacketInfoChan(packetInfoChan)

	fmt.Printf("Shuffle Listening on TCP %s:%d (pool size: %d)\n", ip, port, t.poolSize)
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleConnection(conn, packetInfoChan, t)
	}
}

// StartWebsocket brings up the websocket server.
func StartWebsocket(ip string, port int, cert string, key string, debug bool, t *Tracker, m *autocert.Manager) (err error) {
	packetInfoChan := make(chan *packetInfo)
	go startPacketInfoChan(packetInfoChan)

	var handleConnectionFunc = func(ws *websocket.Conn) {
		// Need to enforce binary type. Text framing won't work.
		ws.PayloadType = websocket.BinaryFrame

		handleConnection(ws, packetInfoChan, t)
	}

	portString := fmt.Sprintf("%s:%d", ip, port)

	mux := http.NewServeMux()
	mux.Handle("/", websocket.Handler(handleConnectionFunc))

	srv := &http.Server{
		Addr:         portString,
		Handler:      mux,
		TLSConfig:    &tls.Config{},
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  5 * time.Minute,
	}

	if m != nil {
		srv.TLSConfig.GetCertificate = m.GetCertificate
	}

	fmt.Printf("Shuffle Listening via Websockets on %s:%d\n", ip, port)

	if tlsEnabled(cert, key, m) {
		err = srv.ListenAndServeTLS(cert, key)
		if err != nil {
			return err
		}
	} else {
		err = srv.ListenAndServe()
		if err != nil {
			return err
		}
	}

	return nil
}

func handleConnection(conn net.Conn, c chan *packetInfo, tracker *Tracker) {
	defer conn.Close()
	defer tracker.remove(conn)

	processMessages(conn, c, tracker)
}
