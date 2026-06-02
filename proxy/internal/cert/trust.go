package cert

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// TrustUser identifies the runtime user whose browser trust store should trust
// the proxy CA.
type TrustUser struct {
	UID      int
	GID      int
	Username string
	HomeDir  string
}

// LookupTrustUser returns the user information needed for NSS trust setup.
func LookupTrustUser(username string) (*TrustUser, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, err
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, fmt.Errorf("invalid uid: %w", err)
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, fmt.Errorf("invalid gid: %w", err)
	}

	return &TrustUser{
		UID:      uid,
		GID:      gid,
		Username: u.Username,
		HomeDir:  u.HomeDir,
	}, nil
}

// InstallTrust installs certPath into browser and system trust stores.
func InstallTrust(certPath string, trustUser *TrustUser) error {
	if err := InstallUserNSSDB(certPath, trustUser); err != nil {
		return err
	}
	return InstallSystemTrust(certPath)
}

// InstallSystemTrust installs the CA certificate in the system trust store.
// It supports Debian/Ubuntu/Alpine and Fedora/RHEL-style trust tooling.
func InstallSystemTrust(certPath string) error {
	fmt.Printf("discobot-proxy: installing proxy CA certificate in system trust store...\n")

	if _, err := exec.LookPath("update-ca-certificates"); err == nil {
		return installCertDebianStyle(certPath)
	}

	if _, err := exec.LookPath("update-ca-trust"); err == nil {
		return installCertFedoraStyle(certPath)
	}

	fmt.Printf("discobot-proxy: warning: no certificate update tool found (update-ca-certificates or update-ca-trust)\n")
	fmt.Printf("discobot-proxy: warning: proxy CA certificate not installed in system trust store\n")
	fmt.Printf("discobot-proxy: warning: HTTPS interception may not work for some clients\n")
	return nil
}

func installCertDebianStyle(certPath string) error {
	const bundlePath = "/etc/ssl/certs/ca-certificates.crt"

	destDir := "/usr/local/share/ca-certificates"
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create ca-certificates dir: %w", err)
	}

	destPath := filepath.Join(destDir, "discobot-proxy-ca.crt")
	data, err := os.ReadFile(filepath.Clean(certPath))
	if err != nil {
		return fmt.Errorf("read certificate: %w", err)
	}

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("write certificate to %s: %w", destPath, err)
	}

	if err := runUpdateCACertificates(); err != nil {
		return err
	}

	installed, err := pemFileContainsCertificate(bundlePath, certPath)
	if err != nil {
		return fmt.Errorf("verify system CA bundle %s: %w", bundlePath, err)
	}
	if !installed {
		fmt.Printf("discobot-proxy: proxy CA certificate missing from %s after update; forcing full rebuild\n", bundlePath)
		if err := runUpdateCACertificates("--fresh"); err != nil {
			return err
		}
		installed, err = pemFileContainsCertificate(bundlePath, certPath)
		if err != nil {
			return fmt.Errorf("verify rebuilt system CA bundle %s: %w", bundlePath, err)
		}
		if !installed {
			return fmt.Errorf("proxy CA certificate still missing from %s after update-ca-certificates --fresh", bundlePath)
		}
	}

	fmt.Printf("discobot-proxy: proxy CA certificate installed in system trust store (Debian/Ubuntu/Alpine)\n")
	return nil
}

func runUpdateCACertificates(args ...string) error {
	cmd := exec.Command("update-ca-certificates", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run update-ca-certificates %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func pemFileContainsCertificate(pemPath, certPath string) (bool, error) {
	certData, err := os.ReadFile(filepath.Clean(certPath))
	if err != nil {
		return false, fmt.Errorf("read certificate %s: %w", certPath, err)
	}
	want, err := parseFirstCertificate(certData)
	if err != nil {
		return false, fmt.Errorf("parse certificate %s: %w", certPath, err)
	}

	pemData, err := os.ReadFile(filepath.Clean(pemPath))
	if err != nil {
		return false, fmt.Errorf("read PEM file %s: %w", pemPath, err)
	}

	for len(pemData) > 0 {
		var block *pem.Block
		block, pemData = pem.Decode(pemData)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		if bytes.Equal(block.Bytes, want.Raw) {
			return true, nil
		}
	}

	return false, nil
}

func parseFirstCertificate(data []byte) (*x509.Certificate, error) {
	for len(data) > 0 {
		block, rest := pem.Decode(data)
		if block == nil {
			break
		}
		data = rest
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		return cert, nil
	}

	return nil, fmt.Errorf("no PEM certificate found")
}

func installCertFedoraStyle(certPath string) error {
	destDir := "/etc/pki/ca-trust/source/anchors"
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create ca-trust dir: %w", err)
	}

	destPath := filepath.Join(destDir, "discobot-proxy-ca.crt")
	data, err := os.ReadFile(filepath.Clean(certPath))
	if err != nil {
		return fmt.Errorf("read certificate: %w", err)
	}

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("write certificate to %s: %w", destPath, err)
	}

	cmd := exec.Command("update-ca-trust", "extract")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run update-ca-trust: %w", err)
	}

	fmt.Printf("discobot-proxy: proxy CA certificate installed in system trust store (Fedora/RHEL)\n")
	return nil
}

// InstallUserNSSDB installs the CA certificate into the runtime user's NSS
// database so Chromium-based browsers trust the proxy certificate.
func InstallUserNSSDB(certPath string, trustUser *TrustUser) error {
	if trustUser == nil {
		return nil
	}

	certutilAvailable := true
	if _, err := exec.LookPath("certutil"); err != nil {
		certutilAvailable = false
	}
	if !certutilAvailable {
		fmt.Printf("discobot-proxy: warning: certutil not found; skipping Chromium/NSS trust setup\n")
		return nil
	}

	nssDBDir := filepath.Join(trustUser.HomeDir, ".pki", "nssdb")
	if err := os.MkdirAll(nssDBDir, 0755); err != nil {
		return fmt.Errorf("create NSS DB directory %s: %w", nssDBDir, err)
	}
	if err := os.Chown(filepath.Join(trustUser.HomeDir, ".pki"), trustUser.UID, trustUser.GID); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("chown NSS parent directory: %w", err)
	}
	if err := os.Chown(nssDBDir, trustUser.UID, trustUser.GID); err != nil {
		return fmt.Errorf("chown NSS DB directory: %w", err)
	}

	nssDB := "sql:" + nssDBDir
	if _, err := os.Stat(filepath.Join(nssDBDir, "cert9.db")); os.IsNotExist(err) {
		cmd := exec.Command("certutil", "-d", nssDB, "-N", "--empty-password")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("initialize NSS DB %s: %w", nssDBDir, err)
		}
	}

	_ = exec.Command("certutil", "-d", nssDB, "-D", "-n", "discobot-proxy-ca").Run()

	addCmd := exec.Command("certutil", "-d", nssDB, "-A", "-t", "C,,", "-n", "discobot-proxy-ca", "-i", filepath.Clean(certPath))
	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("import proxy CA into NSS DB %s: %w", nssDBDir, err)
	}

	if err := chownRecursive(nssDBDir, trustUser.UID, trustUser.GID); err != nil {
		return fmt.Errorf("set ownership on NSS DB %s: %w", nssDBDir, err)
	}

	fmt.Printf("discobot-proxy: proxy CA certificate installed in NSS DB for %s at %s\n", trustUser.Username, nssDBDir)
	return nil
}

func chownRecursive(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(name, uid, gid)
	})
}
