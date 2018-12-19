package server

import (
	"crypto/tls"
	"fmt"
	"net"

	"golang.org/x/crypto/acme/autocert"
)

// createTLSListener creates a net.Listener with TLS support.
func createTLSListener(ip string, port int, cert string, key string, m *autocert.Manager) (net.Listener, error) {
	c := &tls.Config{}

	if cert != "" && key != "" {
		cer, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, err
		}

		c.Certificates = []tls.Certificate{cer}
	} else {
		c.GetCertificate = m.GetCertificate
	}

	listener, err := tls.Listen("tcp", fmt.Sprintf("%s:%d", ip, port), c)
	if err != nil {
		return nil, err
	}

	return listener, nil
}

// tlsEnabled returns a bool indicating if TLS should be supported.
func tlsEnabled(cert string, key string, autoCertManager *autocert.Manager) bool {
	if autoCertManager != nil || (cert != "" && key != "") {
		return true
	}

	return false
}
