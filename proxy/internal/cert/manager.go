// Package cert provides certificate management for MITM proxying.
package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Manager manages CA certificates for MITM proxying.
type Manager struct {
	certDir string
	ca      *tls.Certificate
}

// NewManager creates a new certificate manager.
func NewManager(certDir string) (*Manager, error) {
	certDir = filepath.Clean(certDir)
	m := &Manager{certDir: certDir}

	ca, err := m.getOrCreateCA()
	if err != nil {
		return nil, err
	}
	m.ca = ca

	return m, nil
}

// GetCA returns the CA certificate.
func (m *Manager) GetCA() *tls.Certificate {
	return m.ca
}

// GetCACertPath returns the path to the CA certificate file.
func (m *Manager) GetCACertPath() string {
	return filepath.Join(m.certDir, "ca.crt")
}

func (m *Manager) getOrCreateCA() (*tls.Certificate, error) {
	certPath := filepath.Join(m.certDir, "ca.crt")
	keyPath := filepath.Join(m.certDir, "ca.key")

	if _, _, err := EnsureCA(m.certDir); err != nil {
		return nil, err
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load CA certificate: %w", err)
	}

	return &cert, nil
}

// EnsureCA creates a proxy CA certificate and private key if the configured
// pair does not already exist or cannot be loaded.
func EnsureCA(certDir string) (certPath string, generated bool, err error) {
	certDir = filepath.Clean(certDir)
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return "", false, fmt.Errorf("create cert dir: %w", err)
	}
	if err := os.Chmod(certDir, 0755); err != nil {
		return "", false, fmt.Errorf("chmod cert dir: %w", err)
	}

	certPath = filepath.Join(certDir, "ca.crt")
	keyPath := filepath.Join(certDir, "ca.key")
	if usable, err := hasUsableCA(certPath, keyPath); err != nil {
		return "", false, err
	} else if usable {
		return certPath, false, nil
	}

	if err := generateCA(certPath, keyPath); err != nil {
		return "", false, err
	}

	return certPath, true, nil
}

func hasUsableCA(certPath, keyPath string) (bool, error) {
	if _, err := os.Stat(certPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat CA certificate: %w", err)
	}

	if _, err := os.Stat(keyPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat CA key: %w", err)
	}

	if _, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		return true, nil
	}

	return false, nil
}

func generateCA(certPath, keyPath string) error {
	certPath = filepath.Clean(certPath)
	keyPath = filepath.Clean(keyPath)

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	// Generate serial number
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Discobot Proxy"},
			CommonName:   "Discobot Proxy CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	// Save certificate
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create cert file: %w", err)
	}
	defer func() { _ = certFile.Close() }()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("encode cert: %w", err)
	}
	if err := os.Chmod(certPath, 0644); err != nil {
		return fmt.Errorf("chmod cert file: %w", err)
	}

	// Save private key
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	defer func() { _ = keyFile.Close() }()

	keyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("encode key: %w", err)
	}
	if err := os.Chmod(keyPath, 0600); err != nil {
		return fmt.Errorf("chmod key file: %w", err)
	}

	return nil
}
