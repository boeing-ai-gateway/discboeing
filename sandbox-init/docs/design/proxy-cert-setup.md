# Proxy CA Certificate - Automatic Setup

## Overview

During container startup, sandbox-init runs `proxy init-certs` so the proxy binary generates a CA certificate and installs it in:

- the system trust store
- the runtime user's NSS database (`~/.pki/nssdb`) for Chromium-based browsers

This enables transparent HTTPS interception without certificate warnings in both CLI clients and Chromium automation.

## Why This Is Needed

For the proxy to cache HTTPS traffic (like Docker registry pulls), it must perform "Man-in-the-Middle" (MITM) interception:

1. Client makes HTTPS request to `registry-1.docker.io`
2. Proxy intercepts the connection
3. Proxy generates a fake certificate for `registry-1.docker.io` **signed by its CA**
4. Proxy forwards the request to the real server
5. Client verifies the fake certificate using the **trusted CA**

Without the CA in the appropriate trust stores, clients would see certificate errors and refuse to connect. In particular, Chromium-based browsers may require the certificate to be present in the user's NSS database even when the system trust store is configured.

## Implementation

### Step 1: Certificate Generation

**Location**:

- `proxy/cmd/proxy/main.go` - `init-certs` CLI subcommand
- `proxy/internal/cert/manager.go` - `EnsureCA()` and CA generation

**Process**:

```bash
/opt/discobot/bin/proxy init-certs \
  -config /.data/proxy/config.yaml \
  -user discobot
```

The subcommand loads `tls.cert_dir` from the proxy config, checks for a
loadable `ca.crt`/`ca.key` pair, and generates a replacement pair when either
file is missing or unusable. The proxy server uses the same `EnsureCA` path as
a startup fallback, but trust-store installation is intentionally handled by the
CLI subcommand.

**Key Details**:
- **Implementation**: Pure Go using `crypto/x509` and `crypto/rsa` (no external dependencies)
- **Subject**: `O=Discobot Proxy, CN=Discobot Proxy CA`
- **SANs (Subject Alternative Names)**: `localhost`, `127.0.0.1`, `::1`
- **Validity**: 10 years (3650 days)
- **Key Size**: 2048-bit RSA
- **Storage**: `/.data/proxy/certs/` (persistent if `/.data` is a volume)
- **Reuse**: Existing usable certificate/key pairs are reused

### Step 2: Browser and System Trust Installation

**Chromium / NSS**:

The proxy subcommand imports the proxy CA into the runtime user's NSS database with `certutil`:

```bash
mkdir -p /home/discobot/.pki/nssdb
certutil -d sql:/home/discobot/.pki/nssdb -N --empty-password
certutil -d sql:/home/discobot/.pki/nssdb -A \
  -t "C,," \
  -n discobot-proxy-ca \
  -i /.data/proxy/certs/ca.crt
```

This is the trust path Chromium-based browsers actually consult in this environment.

**Location**: `proxy/internal/cert/trust.go` - `InstallSystemTrust()` and `InstallUserNSSDB()`

**Detection Logic**:
```go
// Try Debian/Ubuntu/Alpine style first
if _, err := exec.LookPath("update-ca-certificates"); err == nil {
    return installCertDebianStyle(certPath)
}

// Try Fedora/RHEL style second
if _, err := exec.LookPath("update-ca-trust"); err == nil {
    return installCertFedoraStyle(certPath)
}

// No tool found - warn but continue
fmt.Printf("warning: no certificate update tool found\n")
return nil
```

### Step 3a: Debian/Ubuntu/Alpine Installation

**Location**: `installCertDebianStyle()`

**Process**:
```bash
# Copy certificate to standard location
cp /.data/proxy/certs/ca.crt /usr/local/share/ca-certificates/discobot-proxy-ca.crt

# Update system trust store
update-ca-certificates
```

After updating, the proxy subcommand verifies that the exact proxy CA certificate is
present in `/etc/ssl/certs/ca-certificates.crt`. If the bundle is still stale
after a normal update, the agent forces a full rebuild with
`update-ca-certificates --fresh` before continuing.

**What happens**:
- Certificate is added to `/etc/ssl/certs/ca-certificates.crt` (bundle)
- All programs using OpenSSL/GnuTLS automatically trust it
- Docker, curl, wget, Python requests, Node.js https, etc. all work

### Step 3b: Fedora/RHEL/CentOS Installation

**Location**: `installCertFedoraStyle()`

**Process**:
```bash
# Copy certificate to standard location
cp /.data/proxy/certs/ca.crt /etc/pki/ca-trust/source/anchors/discobot-proxy-ca.crt

# Update system trust store
update-ca-trust extract
```

**What happens**:
- Certificate is processed into various trust bundle formats
- Trust bundles updated: `/etc/pki/ca-trust/extracted/`
- All programs using NSS/OpenSSL automatically trust it

## Startup Flow Integration

The certificate setup happens during container initialization:

```
Container Startup
    ↓
[Steps 1-5: Home, workspace, filesystem setup]
    ↓
Step 5b: Setup proxy config from embedded defaults
    ↓
Step 5c: Run proxy init-certs
    ├─ Load /.data/proxy/config.yaml
    ├─ Generate or reuse /.data/proxy/certs/ca.{crt,key}
    ├─ Import CA into the runtime user's NSS DB
    ├─ Detect OS (Debian/Fedora/Alpine/other)
    ├─ Copy to system trust directory
    └─ Run update command (update-ca-certificates or update-ca-trust)
    ↓
Step 6: Start proxy daemon
    ├─ Proxy loads CA from /.data/proxy/certs/
    └─ Proxy can now sign fake certificates for HTTPS MITM
    ↓
Step 7: Start Docker daemon (with proxy env vars)
    └─ Docker trusts proxy CA, no certificate errors
    ↓
Step 8: Start agent-api (with proxy env vars)
    └─ Agent API trusts proxy CA, no certificate errors
```

## Benefits

### Before (Manual Trust)
```
❌ User must manually extract CA cert from container
❌ User must manually install in host system
❌ Agent processes see certificate errors for HTTPS
❌ Docker pull may fail or bypass proxy
❌ Complex setup for developers
```

### After (Automatic Trust)
```
✅ CA certificate auto-generated on first run
✅ Automatically installed in container system trust
✅ All processes in container trust the CA
✅ Docker pulls work seamlessly through proxy
✅ HTTPS caching works out of the box
✅ Zero configuration required
```

## Security Considerations

### Private Key Protection
- Key file: `/.data/proxy/certs/ca.key`
- Permissions: `0600` (owner read/write only)
- Owner: root (the init command runs as root)
- Not exposed outside container

### Certificate Trust Scope
- **Container only**: Trust is limited to the container's system trust store
- **Host unaffected**: Host system trust store is not modified
- **Volume persistence**: If `/.data` is a volume, the same CA is reused across restarts as long as both `/.data/proxy/certs/ca.crt` and `ca.key` remain present and loadable as a pair

### Certificate Rotation
- **10-year validity**: Long enough to avoid frequent rotation
- **Manual rotation**: Delete `/.data/proxy/certs/ca.{crt,key}` and restart container
- **Automatic regeneration**: New certificate generated if files deleted

## Testing

### Verify Certificate Generation
```bash
# Enter running container
docker exec -it <container> bash

# Check certificate exists
ls -la /.data/proxy/certs/
# Should show:
# -rw-r--r-- 1 root root 1188 ... ca.crt
# -rw------- 1 root root 1675 ... ca.key

# View certificate details
openssl x509 -in /.data/proxy/certs/ca.crt -text -noout
# Should show:
#   Subject: O = Discobot Proxy, CN = Discobot Proxy CA
#   Validity: Not After : <10 years from generation>
```

### Verify System Trust (Debian/Ubuntu/Alpine)
```bash
# Check certificate in trust directory
ls -la /usr/local/share/ca-certificates/discobot-proxy-ca.crt

# Check certificate in bundle
grep -q "Discobot Proxy" /etc/ssl/certs/ca-certificates.crt && echo "FOUND"
```

### Verify System Trust (Fedora/RHEL)
```bash
# Check certificate in trust directory
ls -la /etc/pki/ca-trust/source/anchors/discobot-proxy-ca.crt

# Check trust bundles updated
ls -la /etc/pki/ca-trust/extracted/
```

### Verify HTTPS Works Through Proxy
```bash
# Set proxy environment
export HTTP_PROXY=http://localhost:17080
export HTTPS_PROXY=http://localhost:17080

# Test HTTPS request (should work without certificate errors)
curl -v https://registry-1.docker.io/v2/
# Should see "HTTP/1.1 200 OK" or "HTTP/1.1 401 Unauthorized" (auth required)
# Should NOT see certificate errors

# Test Docker pull through proxy
docker pull ubuntu:latest
# Should work without certificate warnings
```

## Troubleshooting

### Certificate Not Generated
**Symptom**: No certificate at `/.data/proxy/certs/ca.crt`

**Possible causes**:
- Permission issues creating `/.data/proxy/certs/`
- Startup step skipped due to earlier error
- Go crypto library error (RSA key generation or certificate creation)

**Solution**: Check sandbox-init logs for `proxy init-certs` errors during Step 5c

### Certificate Not Trusted
**Symptom**: Certificate errors when making HTTPS requests through proxy

**Possible causes**:
- Certificate update tool not found (unsupported OS)
- Update command failed (check logs)
- Program using custom trust store (bypassing system)

**Debug commands**:
```bash
# Check which update tool is available
which update-ca-certificates update-ca-trust

# Manually run update
update-ca-certificates         # Debian/Ubuntu/Alpine
update-ca-trust extract        # Fedora/RHEL

# Check if certificate is in bundle
grep "Discobot Proxy" /etc/ssl/certs/ca-certificates.crt
```

### Certificate Generation Fails
**Symptom**: Logs show errors like "generate key" or "create certificate"

**Possible causes**:
- Insufficient entropy for random number generation (rare in containers)
- Disk write failure (out of space or permissions)

**Solution**:
- Check available disk space: `df -h /.data`
- Verify directory permissions: `ls -ld /.data/proxy/certs`
- Check dmesg for kernel-level errors

## Files Modified

### Proxy Code
- **`proxy/cmd/proxy/main.go`**:
  - Added `init-certs` subcommand.
  - Loads proxy config, initializes the CA, and installs trust stores.
- **`proxy/internal/cert/manager.go`**:
  - Added reusable `EnsureCA()` generation/reuse logic.
- **`proxy/internal/cert/trust.go`**:
  - Owns system trust and runtime-user NSS setup.

### Sandbox Init Code
- **`sandbox-init/cmd/sandbox-init/main.go`**:
  - Removed inline certificate generation and trust-store implementation.
  - Delegates certificate setup to `/opt/discobot/bin/proxy init-certs`.

## References

- [Go x509 package](https://pkg.go.dev/crypto/x509)
- [Debian CA Certificates](https://wiki.debian.org/Self-Signed_Certificate)
- [RHEL CA Trust](https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/8/html/security_hardening/using-shared-system-certificates_security-hardening)
- [Alpine CA Certificates](https://wiki.alpinelinux.org/wiki/Setting_up_a_Certificate_Authority)
