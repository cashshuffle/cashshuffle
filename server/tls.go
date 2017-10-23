package server

import (
	"crypto/tls"
	"fmt"
	"net"
)

func createTLSListener(port int, cert string, key string) (net.Listener, error) {
	cer, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	c := &tls.Config{Certificates: []tls.Certificate{cer}}
	listener, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), c)
	if err != nil {
		return nil, err
	}

	return listener, nil
}

func tlsEnabled(cert string, key string) bool {
	if cert != "" && key != "" {
		return true
	}

	return false
}
