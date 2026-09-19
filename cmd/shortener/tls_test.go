package main

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelfSignedCert(t *testing.T) {
	cert, err := selfSignedCert()
	require.NoError(t, err)
	require.NotEmpty(t, cert.Certificate)

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)

	assert.Contains(t, leaf.DNSNames, "localhost")
	assert.Contains(t, leaf.IPAddresses[0].String(), "127.0.0.1")

	assert.NoError(t, leaf.VerifyHostname("localhost"))
	assert.True(t, time.Now().After(leaf.NotBefore))
	assert.True(t, time.Now().Before(leaf.NotAfter))
}

func TestSelfSignedCert_FreshEachCall(t *testing.T) {
	first, err := selfSignedCert()
	require.NoError(t, err)

	second, err := selfSignedCert()
	require.NoError(t, err)

	assert.NotEqual(t, first.Certificate[0], second.Certificate[0], "each call should generate a new key/certificate")
}
