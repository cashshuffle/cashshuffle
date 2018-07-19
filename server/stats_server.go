package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/crypto/acme/autocert"
)

// StartStatsServer creates a new server to serve stats
func StartStatsServer(port int, cert string, key string, si StatsInformer, m *autocert.Manager) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", statsJSON(si))
	s := newStatsServer(fmt.Sprintf(":%d", port), mux, m)
	tls := tlsEnabled(cert, key, m)
	fmt.Printf("Stats Listening on TCP port %d (tls: %v)\n", port, tls)
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
		Addr:      addr,
		Handler:   mux,
		TLSConfig: &tls.Config{},
	}

	if m != nil {
		srv.TLSConfig.GetCertificate = m.GetCertificate
	}

	return srv
}
