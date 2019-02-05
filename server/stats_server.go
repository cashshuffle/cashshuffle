package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// StartStatsServer creates a new server to serve stats
func StartStatsServer(ip string, port int, cert string, key string, si StatsInformer, m *autocert.Manager) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", statsJSON(si))
	s := newStatsServer(fmt.Sprintf("%s:%d", ip, port), mux, m)
	tls := tlsEnabled(cert, key, m)
	fmt.Printf("Stats Listening on TCP %s:%d (tls: %v)\n", ip, port, tls)
	if tls {
		return s.ListenAndServeTLS(cert, key)
	}
	return s.ListenAndServe()
}

func statsJSON(si StatsInformer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(si.Stats())
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
}

func newStatsServer(addr string, mux *http.ServeMux, m *autocert.Manager) *http.Server {
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		TLSConfig:    &tls.Config{},
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  5 * time.Minute,
	}

	if m != nil {
		srv.TLSConfig.GetCertificate = m.GetCertificate
	}

	return srv
}
