package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
)

// StartStatsServer creates a new server to serve stats
func StartStatsServer(port int, cert string, key string, si StatsInformer) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", statsJSON(si))
	s := newStatsServer(fmt.Sprintf(":%d", port), mux)
	tls := tlsEnabled(cert, key)
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

func newStatsServer(addr string, mux *http.ServeMux) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		},
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
	}
}
