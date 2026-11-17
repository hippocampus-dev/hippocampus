package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestCanonicalHost(t *testing.T) {
	tests := []struct {
		name      string
		authority string
		scheme    string
		want      string
	}{
		{name: "https default port", authority: "example.com:443", scheme: "https", want: "example.com"},
		{name: "https other port", authority: "example.com:8443", scheme: "https", want: "example.com:8443"},
		{name: "http default port", authority: "example.com:80", scheme: "http", want: "example.com"},
		{name: "http other port", authority: "example.com:8080", scheme: "http", want: "example.com:8080"},
		{name: "no port", authority: "example.com", scheme: "https", want: "example.com"},
		{name: "ipv6 default port", authority: "[::1]:443", scheme: "https", want: "[::1]"},
		{name: "ipv6 other port", authority: "[::1]:8443", scheme: "https", want: "[::1]:8443"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canonicalHost(tt.authority, tt.scheme)
			if got != tt.want {
				t.Errorf("canonicalHost(%q, %q) = %q, want %q", tt.authority, tt.scheme, got, tt.want)
			}
		})
	}
}

func certificateAuthorityPEM(t *testing.T, commonName string, notAfter time.Time) ([]byte, []byte) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, privateKey.Public(), privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	privateKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKeyBytes})
}

func writeCertificateAuthority(t *testing.T, notAfter time.Time) (string, string) {
	t.Helper()

	certificatePEM, privateKeyPEM := certificateAuthorityPEM(t, "tls-intercept-proxy-test", notAfter)

	directory := t.TempDir()
	certificateFile := filepath.Join(directory, "tls.crt")
	privateKeyFile := filepath.Join(directory, "tls.key")

	if err := os.WriteFile(certificateFile, certificatePEM, 0o600); err != nil {
		t.Fatalf("failed to write certificate: %v", err)
	}
	if err := os.WriteFile(privateKeyFile, privateKeyPEM, 0o600); err != nil {
		t.Fatalf("failed to write private key: %v", err)
	}

	return certificateFile, privateKeyFile
}

func TestCertificateAuthorityIssue(t *testing.T) {
	tests := []struct {
		name            string
		host            string
		wantDNSNames    []string
		wantIPAddresses []string
	}{
		{name: "dns name", host: "example.com", wantDNSNames: []string{"example.com"}},
		{name: "ipv4 address", host: "192.0.2.1", wantIPAddresses: []string{"192.0.2.1"}},
		{name: "ipv6 address", host: "::1", wantIPAddresses: []string{"::1"}},
	}

	certificateFile, privateKeyFile := writeCertificateAuthority(t, time.Now().Add(24*time.Hour))

	authority, err := NewCertificateAuthority(certificateFile, privateKeyFile, time.Hour)
	if err != nil {
		t.Fatalf("failed to create certificate authority: %v", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(authority.certificate)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issued, err := authority.Issue(tt.host)
			if err != nil {
				t.Fatalf("failed to issue certificate: %v", err)
			}

			certificate, err := x509.ParseCertificate(issued.Certificate[0])
			if err != nil {
				t.Fatalf("failed to parse issued certificate: %v", err)
			}

			if !slices.Equal(certificate.DNSNames, tt.wantDNSNames) {
				t.Errorf("DNSNames = %v, want %v", certificate.DNSNames, tt.wantDNSNames)
			}

			addresses := []string{}
			for _, address := range certificate.IPAddresses {
				addresses = append(addresses, address.String())
			}
			if !slices.Equal(addresses, tt.wantIPAddresses) {
				t.Errorf("IPAddresses = %v, want %v", addresses, tt.wantIPAddresses)
			}

			if _, err := certificate.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
				t.Errorf("failed to verify issued certificate: %v", err)
			}

			again, err := authority.Issue(tt.host)
			if err != nil {
				t.Fatalf("failed to issue certificate: %v", err)
			}
			if again != issued {
				t.Errorf("Issue(%q) returned a new certificate instead of the cached one", tt.host)
			}
		})
	}
}

func TestCertificateAuthorityIssueReplacesExpired(t *testing.T) {
	certificateFile, privateKeyFile := writeCertificateAuthority(t, time.Now().Add(24*time.Hour))

	authority, err := NewCertificateAuthority(certificateFile, privateKeyFile, 0)
	if err != nil {
		t.Fatalf("failed to create certificate authority: %v", err)
	}

	issued, err := authority.Issue("example.com")
	if err != nil {
		t.Fatalf("failed to issue certificate: %v", err)
	}

	again, err := authority.Issue("example.com")
	if err != nil {
		t.Fatalf("failed to issue certificate: %v", err)
	}
	if again == issued {
		t.Error("Issue returned the expired cached certificate")
	}
}

func TestCertificateAuthorityIssueReloadsReplacedCertificateAuthority(t *testing.T) {
	certificateFile, privateKeyFile := writeCertificateAuthority(t, time.Now().Add(24*time.Hour))

	authority, err := NewCertificateAuthority(certificateFile, privateKeyFile, time.Hour)
	if err != nil {
		t.Fatalf("failed to create certificate authority: %v", err)
	}

	issued, err := authority.Issue("example.com")
	if err != nil {
		t.Fatalf("failed to issue certificate: %v", err)
	}
	before, err := x509.ParseCertificate(issued.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse issued certificate: %v", err)
	}

	certificatePEM, privateKeyPEM := certificateAuthorityPEM(t, "tls-intercept-proxy-test-renewed", time.Now().Add(24*time.Hour))
	if err := os.WriteFile(certificateFile, certificatePEM, 0o600); err != nil {
		t.Fatalf("failed to write certificate: %v", err)
	}
	if err := os.WriteFile(privateKeyFile, privateKeyPEM, 0o600); err != nil {
		t.Fatalf("failed to write private key: %v", err)
	}
	modified := time.Now().Add(time.Second)
	if err := os.Chtimes(certificateFile, modified, modified); err != nil {
		t.Fatalf("failed to change certificate modification time: %v", err)
	}

	reissued, err := authority.Issue("example.com")
	if err != nil {
		t.Fatalf("failed to issue certificate: %v", err)
	}
	after, err := x509.ParseCertificate(reissued.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse issued certificate: %v", err)
	}

	if before.Issuer.CommonName != "tls-intercept-proxy-test" {
		t.Errorf("Issuer.CommonName = %q, want %q", before.Issuer.CommonName, "tls-intercept-proxy-test")
	}
	if after.Issuer.CommonName != "tls-intercept-proxy-test-renewed" {
		t.Errorf("Issuer.CommonName = %q, want %q", after.Issuer.CommonName, "tls-intercept-proxy-test-renewed")
	}
}

func TestCertificateAuthorityIssueClampsLifetime(t *testing.T) {
	notAfter := time.Now().Add(30 * time.Minute)
	certificateFile, privateKeyFile := writeCertificateAuthority(t, notAfter)

	authority, err := NewCertificateAuthority(certificateFile, privateKeyFile, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to create certificate authority: %v", err)
	}

	issued, err := authority.Issue("example.com")
	if err != nil {
		t.Fatalf("failed to issue certificate: %v", err)
	}

	certificate, err := x509.ParseCertificate(issued.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse issued certificate: %v", err)
	}

	if certificate.NotAfter.After(authority.certificate.NotAfter) {
		t.Errorf("NotAfter = %v, want at most %v", certificate.NotAfter, authority.certificate.NotAfter)
	}
}

func TestCertificateAuthorityIssueConcurrency(t *testing.T) {
	certificateFile, privateKeyFile := writeCertificateAuthority(t, time.Now().Add(24*time.Hour))

	authority, err := NewCertificateAuthority(certificateFile, privateKeyFile, time.Hour)
	if err != nil {
		t.Fatalf("failed to create certificate authority: %v", err)
	}

	hosts := []string{"example.com", "example.org", "example.net"}
	rounds := 32

	var waitGroup sync.WaitGroup
	failures := make(chan error, len(hosts)*rounds)
	for range rounds {
		for _, host := range hosts {
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()

				issued, err := authority.Issue(host)
				if err != nil {
					failures <- err
					return
				}
				certificate, err := x509.ParseCertificate(issued.Certificate[0])
				if err != nil {
					failures <- err
					return
				}
				if !slices.Contains(certificate.DNSNames, host) {
					failures <- fmt.Errorf("DNSNames = %v, want to contain %q", certificate.DNSNames, host)
				}
			}()
		}
	}
	waitGroup.Wait()
	close(failures)

	for failure := range failures {
		t.Error(failure)
	}
}
