package cert

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCAReturnsExistingValidPair(t *testing.T) {
	certDir := t.TempDir()

	certPath, generated, err := EnsureCA(certDir)
	if err != nil {
		t.Fatalf("EnsureCA failed: %v", err)
	}
	if !generated {
		t.Fatal("EnsureCA generated = false, want true")
	}

	again, generated, err := EnsureCA(certDir)
	if err != nil {
		t.Fatalf("EnsureCA second call failed: %v", err)
	}
	if generated {
		t.Fatal("EnsureCA second call generated = true, want false")
	}
	if again != certPath {
		t.Fatalf("EnsureCA cert path = %q, want %q", again, certPath)
	}
}

func TestEnsureCARegeneratesWhenKeyMissing(t *testing.T) {
	certDir := t.TempDir()

	if _, _, err := EnsureCA(certDir); err != nil {
		t.Fatalf("EnsureCA failed: %v", err)
	}
	if err := os.Remove(filepath.Join(certDir, "ca.key")); err != nil {
		t.Fatalf("remove key: %v", err)
	}

	if _, generated, err := EnsureCA(certDir); err != nil {
		t.Fatalf("EnsureCA after key removal failed: %v", err)
	} else if !generated {
		t.Fatal("EnsureCA generated = false, want true")
	}
}

func TestPEMFileContainsCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath, _, err := EnsureCA(filepath.Join(dir, "cert"))
	if err != nil {
		t.Fatalf("EnsureCA(cert) failed: %v", err)
	}
	otherCertPath, _, err := EnsureCA(filepath.Join(dir, "other"))
	if err != nil {
		t.Fatalf("EnsureCA(other) failed: %v", err)
	}
	bundlePath := filepath.Join(dir, "bundle.pem")

	certData, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read certPath: %v", err)
	}
	otherCertData, err := os.ReadFile(otherCertPath)
	if err != nil {
		t.Fatalf("read otherCertPath: %v", err)
	}

	if err := os.WriteFile(bundlePath, append(otherCertData, certData...), 0644); err != nil {
		t.Fatalf("write bundlePath: %v", err)
	}

	contains, err := pemFileContainsCertificate(bundlePath, certPath)
	if err != nil {
		t.Fatalf("pemFileContainsCertificate returned error: %v", err)
	}
	if !contains {
		t.Fatal("pemFileContainsCertificate = false, want true")
	}

	contains, err = pemFileContainsCertificate(bundlePath, otherCertPath)
	if err != nil {
		t.Fatalf("pemFileContainsCertificate(other) returned error: %v", err)
	}
	if !contains {
		t.Fatal("pemFileContainsCertificate(other) = false, want true")
	}
}

func TestPEMFileContainsCertificateReturnsFalseWhenMissing(t *testing.T) {
	dir := t.TempDir()
	certPath, _, err := EnsureCA(filepath.Join(dir, "cert"))
	if err != nil {
		t.Fatalf("EnsureCA(cert) failed: %v", err)
	}
	otherCertPath, _, err := EnsureCA(filepath.Join(dir, "other"))
	if err != nil {
		t.Fatalf("EnsureCA(other) failed: %v", err)
	}
	bundlePath := filepath.Join(dir, "bundle.pem")

	otherCertData, err := os.ReadFile(otherCertPath)
	if err != nil {
		t.Fatalf("read otherCertPath: %v", err)
	}
	if err := os.WriteFile(bundlePath, otherCertData, 0644); err != nil {
		t.Fatalf("write bundlePath: %v", err)
	}

	contains, err := pemFileContainsCertificate(bundlePath, certPath)
	if err != nil {
		t.Fatalf("pemFileContainsCertificate returned error: %v", err)
	}
	if contains {
		t.Fatal("pemFileContainsCertificate = true, want false")
	}
}
