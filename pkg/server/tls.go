package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// PeerTLSConfig uses a dedicated CA for verifying federation peers.
func PeerTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	if certFile == "" && keyFile == "" && caFile == "" {
		return nil, nil
	}
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("peer CA file contains no certificates")
	}
	return &tls.Config{Certificates: []tls.Certificate{pair}, ClientCAs: roots, ClientAuth: tls.VerifyClientCertIfGiven, MinVersion: tls.VersionTLS13}, nil
}
