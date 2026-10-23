package cloudflare

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"

	"github.com/cockroachdb/errors"
)

// OriginValidityDays is the longest validity Origin CA offers (15 years), so
// the certificate never has to be renewed on the nodes.
const OriginValidityDays = 5475

// OriginHostnames returns the names an Origin certificate for a zone covers:
// the apex and every direct subdomain, so rotated subdomains need no new one.
func OriginHostnames(domain string) []string {
	return []string{domain, "*." + domain}
}

// GenerateCsr makes a fresh ECDSA P-256 key and a CSR for the hostnames, both
// PEM-encoded, ready for CreateOriginCertificate.
func GenerateCsr(hostnames []string) (privateKey, csr string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", errors.WithStack(err)
	}
	keyDer, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	template := &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: hostnames[0]},
		DNSNames: hostnames,
	}
	csrDer, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	privateKey = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer}))
	csr = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDer}))
	return privateKey, csr, nil
}
