package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	"golang.org/x/crypto/acme/autocert"

	log "github.com/sirupsen/logrus"
)

// StartStatsServer creates a new server to serve stats
func StartStatsServer(ip string, port int, cert string, key string, si StatsInformer, m *autocert.Manager, tor bool, limit *limiter.Limiter) error {
	mux := http.NewServeMux()
	middleware := stdlib.NewMiddleware(limit)
	statsJSONHandler := http.HandlerFunc(statsJSON(si, tor))
	mux.Handle("/stats", middleware.Handler(statsJSONHandler))
	s := newStatsServer(fmt.Sprintf("%s:%d", ip, port), mux, m)
	isTLS := tlsEnabled(cert, key, m)

	torStr := ""
	if tor {
		torStr = "Tor"
	}

	log.Infof(logListener+"%sStats Listening on TCP %s:%d (tls: %v)\n", torStr, ip, port, isTLS)
	if isTLS {
		return s.ListenAndServeTLS(cert, key)
	}
	return s.ListenAndServe()
}

func statsJSON(si StatsInformer, tor bool) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		b, _ := json.Marshal(si.Stats(ip, tor))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
		w.Write(b)
	}
}

func newStatsServer(addr string, mux *http.ServeMux, m *autocert.Manager) *http.Server {
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		TLSConfig:    &tls.Config{},
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	if m != nil {
		srv.TLSConfig.GetCertificate = m.GetCertificate
	}

	return srv
}
