package kube

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "steloit-test-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(100 * 365 * 24 * time.Hour),
		IsCA: true, KeyUsage: x509.KeyUsageCertSign, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func saDirWith(t *testing.T, token string, ca []byte) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	if ca != nil {
		if err := os.WriteFile(filepath.Join(dir, "ca.crt"), ca, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prev := saDir
	saDir = dir
	t.Cleanup(func() { saDir = prev })
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "443")
	return dir
}

// THE PRODUCTION CONSTRUCTOR must wire the token as a FILE, not cache its
// contents.
//
// TestAuthRereadsTheRotatedServiceAccountToken proves the re-read mechanism, but
// it builds its client with NewClientForTest and sets tokenFile by hand — so it
// cannot see this wiring at all, and NewInCluster could go back to caching the
// token at boot with the whole module green. Two representations of "the token is
// re-read": the mechanism and the wiring. That test covered the first.
//
// What caching costs is in the type's own comment: GKE projected tokens expire
// (~1h) and are rotated in place, so every apply 401s after the TTL and never
// recovers without a restart.
func TestNewInClusterWiresTheTokenFileRatherThanCachingIt(t *testing.T) {
	saDirWith(t, "boot-token", testCAPEM(t))

	c, err := NewInCluster()
	if err != nil {
		t.Fatal(err)
	}
	if c.tokenFile == "" {
		t.Fatal("NewInCluster did not wire tokenFile — the token is read once at boot and " +
			"every apply 401s after the projected token's ~1h TTL, never recovering")
	}
	if c.token != "" {
		t.Fatalf("NewInCluster cached the token value (%q); the file is the source of truth", c.token)
	}
	if got := filepath.Base(c.tokenFile); got != "token" {
		t.Fatalf("tokenFile is %q", c.tokenFile)
	}
}

// The cluster CA must be read AND validated. Deleting either was green: with no
// CA the agent takes the in-cluster branch with an empty pool, logs "real apply",
// and every converge fails TLS — a working-looking agent that provisions nothing.
func TestNewInClusterRefusesAnUnusableClusterCA(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		saDirWith(t, "tok", nil)
		if _, err := NewInCluster(); err == nil {
			t.Fatal("NewInCluster accepted a missing ca.crt")
		}
	})
	t.Run("not PEM", func(t *testing.T) {
		saDirWith(t, "tok", []byte("this is not a certificate"))
		if _, err := NewInCluster(); err == nil {
			t.Fatal("NewInCluster accepted a ca.crt that is not valid PEM — the RootCAs pool " +
				"would be empty and every converge would fail TLS behind a 'real apply' log line")
		}
	})
	t.Run("valid", func(t *testing.T) {
		saDirWith(t, "tok", testCAPEM(t))
		if _, err := NewInCluster(); err != nil {
			t.Fatalf("NewInCluster refused a valid CA: %v", err)
		}
	})
	t.Run("no cluster", func(t *testing.T) {
		saDirWith(t, "tok", testCAPEM(t))
		t.Setenv("KUBERNETES_SERVICE_HOST", "")
		if _, err := NewInCluster(); err == nil {
			t.Fatal("NewInCluster built a client with no KUBERNETES_SERVICE_HOST")
		}
	})
}
