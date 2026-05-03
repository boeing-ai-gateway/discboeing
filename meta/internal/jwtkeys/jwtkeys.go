package jwtkeys

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/id"
	"github.com/obot-platform/discobot/meta/internal/model"
	"github.com/obot-platform/discobot/meta/internal/store"
)

const (
	AlgorithmES256 = "ES256"
	BackendDBLocal = model.JWTSigningKeyBackendDBLocal
	BackendGCPKMS  = model.JWTSigningKeyBackendGCPKMS
)

var (
	ErrUnsupportedSignerBackend = errors.New("unsupported JWT signer backend")
	ErrNoActiveSigningKey       = errors.New("no active JWT signing key")
)

// SigningKeyStore owns JWT signing key lifecycle and public JWKS metadata.
type SigningKeyStore interface {
	ActiveSigningKey(ctx context.Context, issuer string) (*model.JWTSigningKey, error)
	PublicJWKS(ctx context.Context, issuer string) (json.RawMessage, error)
	CreateNextKey(ctx context.Context, issuer string, alg string) (*model.JWTSigningKey, error)
	PromoteKey(ctx context.Context, keyID string) error
	RetireKey(ctx context.Context, keyID string) error
	DisableKey(ctx context.Context, keyID string) error
}

// JWTSigner signs JWT signing input with one signing key backend.
type JWTSigner interface {
	Sign(ctx context.Context, key *model.JWTSigningKey, signingInput []byte) ([]byte, error)
}

// Manager dispatches signing operations to the configured signer backend.
type Manager struct {
	signers map[string]JWTSigner
}

// NewManager returns a signing manager with the provided backend signers.
func NewManager(signers ...JWTSignerRegistration) *Manager {
	m := &Manager{signers: make(map[string]JWTSigner, len(signers))}
	for _, signer := range signers {
		m.signers[signer.Backend] = signer.Signer
	}
	return m
}

// JWTSignerRegistration binds one signer implementation to a backend name.
type JWTSignerRegistration struct {
	Backend string
	Signer  JWTSigner
}

func (m *Manager) Sign(ctx context.Context, key *model.JWTSigningKey, signingInput []byte) ([]byte, error) {
	signer := m.signers[key.Backend]
	if signer == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSignerBackend, key.Backend)
	}
	return signer.Sign(ctx, key, signingInput)
}

// DBLocalSigner signs with private keys encrypted in the Meta database.
type DBLocalSigner struct {
	encryptor dbcrypt.Encryptor
}

func NewDBLocalSigner(encryptor dbcrypt.Encryptor) *DBLocalSigner {
	return &DBLocalSigner{encryptor: encryptor}
}

func (s *DBLocalSigner) Sign(ctx context.Context, key *model.JWTSigningKey, signingInput []byte) ([]byte, error) {
	if key.Backend != BackendDBLocal {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSignerBackend, key.Backend)
	}
	privateKey, err := s.privateKey(ctx, key)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(signingInput)
	r, ss, err := ecdsa.Sign(rand.Reader, privateKey, digest[:])
	if err != nil {
		return nil, fmt.Errorf("sign JWT: %w", err)
	}
	return es256Signature(r, ss), nil
}

func (s *DBLocalSigner) privateKey(ctx context.Context, key *model.JWTSigningKey) (*ecdsa.PrivateKey, error) {
	der, err := store.DecryptField(ctx, s.encryptor, key, store.FieldJWTSigningPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt signing key: %w", err)
	}
	parsed, err := x509.ParseECPrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse signing key: %w", err)
	}
	return parsed, nil
}

// NewDBLocalSigningKey generates a db-local ES256 signing key encrypted for storage.
func NewDBLocalSigningKey(ctx context.Context, encryptor dbcrypt.Encryptor) (*model.JWTSigningKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	der, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshal signing key: %w", err)
	}
	rowID := id.MustNew(id.TypeJWTSigningKey)
	keyID := id.MustNew(id.TypeJWTSigningKey)
	key := &model.JWTSigningKey{
		ID:        rowID,
		KeyID:     keyID,
		Algorithm: AlgorithmES256,
		Backend:   BackendDBLocal,
		Status:    model.JWTSigningKeyStatusNext,
	}
	publicJWK, err := publicJWK(keyID, privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	key.PublicJWKJSON = publicJWK
	if err := store.SetEncryptedField(ctx, encryptor, key, store.FieldJWTSigningPrivateKey, der); err != nil {
		return nil, err
	}
	return key, nil
}

func publicJWK(keyID string, key ecdsa.PublicKey) ([]byte, error) {
	if key.X == nil || key.Y == nil {
		return nil, errors.New("missing public key coordinates")
	}
	return json.Marshal(map[string]string{
		"kty": "EC",
		"crv": "P-256",
		"use": "sig",
		"alg": AlgorithmES256,
		"kid": keyID,
		"x":   encodePadded(key.X),
		"y":   encodePadded(key.Y),
	})
}

func es256Signature(r, s *big.Int) []byte {
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return sig
}

func encodePadded(value *big.Int) string {
	buf := make([]byte, 32)
	value.FillBytes(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}
