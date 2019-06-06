package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/websocket"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		ForceColors: true, // much more readable format for normal use
	})
}

// Start brings up the TCP server.
func Start(ip string, port int, cert string, key string, debug bool, t *Tracker, m *autocert.Manager, tor bool, limit *limiter.Limiter) (err error) {
	var listener net.Listener

	if debug {
		log.SetLevel(log.DebugLevel)
	}

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

	torStr := ""
	if tor {
		torStr = "Tor"
	}

	log.Infof(logListener+"%sShuffle Listening on TCP %s:%d (pool size: %d)\n", torStr, ip, port, t.poolSize)
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		ip := getIP(conn)

		context, err := limit.Get(nil, ip)
		if err != nil {
			log.Debugf(logListener+"Unable to get connection limit: %s\n", err)
			conn.Close()
			continue
		}

		if context.Reached {
			log.Debugf(logListener+"Rate limit exceeded by %s\n", ip)
			conn.Close()
			continue
		}

		go handleConnection(conn, packetInfoChan, t)
	}
}

// StartWebsocket brings up the websocket server.
func StartWebsocket(ip string, port int, cert string, key string, debug bool, t *Tracker, m *autocert.Manager, tor bool, limit *limiter.Limiter) (err error) {
	packetInfoChan := make(chan *packetInfo)
	go startPacketInfoChan(packetInfoChan)

	var handleConnectionFunc = func(ws *websocket.Conn) {
		// Need to enforce binary type. Text framing won't work.
		ws.PayloadType = websocket.BinaryFrame

		handleConnection(ws, packetInfoChan, t)
	}

	portString := fmt.Sprintf("%s:%d", ip, port)

	mux := http.NewServeMux()
	middleware := stdlib.NewMiddleware(limit)
	mux.Handle("/", middleware.Handler(websocket.Handler(handleConnectionFunc)))

	srv := &http.Server{
		Addr:         portString,
		Handler:      mux,
		TLSConfig:    &tls.Config{},
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  deadline,
	}

	if m != nil {
		srv.TLSConfig.GetCertificate = m.GetCertificate
	}

	torStr := ""
	if tor {
		torStr = "Tor"
	}

	log.Infof(logListener+"%sShuffle Listening via Websockets on %s:%d\n", torStr, ip, port)

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

	// They just connected, set the deadline to prevent leaked connections.
	if err := conn.SetDeadline(time.Now().Add(connectDeadline)); err != nil {
		log.Debugf(logCommunication+"Received message but unable to extend deadline: %s\n", err)
	}

	if !tracker.bannedByServer(conn) {
		processMessages(conn, c, tracker)
	}
}

func getIP(conn net.Conn) string {
	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	return ip
}
