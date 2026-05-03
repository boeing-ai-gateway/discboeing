package jwtkeys

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	cloudkms "cloud.google.com/go/kms/apiv1"
	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/googleapis/gax-go/v2"

	"github.com/obot-platform/discobot/meta/internal/id"
	"github.com/obot-platform/discobot/meta/internal/model"
)

type gcpKMSClient interface {
	AsymmetricSign(context.Context, *kmspb.AsymmetricSignRequest, ...gax.CallOption) (*kmspb.AsymmetricSignResponse, error)
	CreateCryptoKeyVersion(context.Context, *kmspb.CreateCryptoKeyVersionRequest, ...gax.CallOption) (*kmspb.CryptoKeyVersion, error)
	GetPublicKey(context.Context, *kmspb.GetPublicKeyRequest, ...gax.CallOption) (*kmspb.PublicKey, error)
}

// GCPKMSSigner signs JWTs with Google Cloud KMS asymmetric key versions.
type GCPKMSSigner struct {
	client gcpKMSClient
}

// NewGCPKMSSigner returns a JWT signer backed by Google Cloud KMS.
func NewGCPKMSSigner(ctx context.Context) (*GCPKMSSigner, error) {
	client, err := cloudkms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create Google Cloud KMS client: %w", err)
	}
	return NewGCPKMSSignerWithClient(client)
}

// NewGCPKMSSignerWithClient returns a GCP KMS signer using an existing client.
func NewGCPKMSSignerWithClient(client gcpKMSClient) (*GCPKMSSigner, error) {
	if client == nil {
		return nil, fmt.Errorf("Google Cloud KMS client is required")
	}
	return &GCPKMSSigner{client: client}, nil
}

// NewGCPKMSSigningKeyFactory returns a key factory that creates new Google
// Cloud KMS CryptoKeyVersions under an admin-created asymmetric signing
// CryptoKey.
func NewGCPKMSSigningKeyFactory(ctx context.Context, cryptoKeyName string) (SigningKeyFactory, error) {
	client, err := cloudkms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create Google Cloud KMS client: %w", err)
	}
	return NewGCPKMSSigningKeyFactoryWithClient(client, cryptoKeyName)
}

// NewGCPKMSSigningKeyFactoryWithClient returns a GCP KMS key factory using an
// existing client.
func NewGCPKMSSigningKeyFactoryWithClient(client gcpKMSClient, cryptoKeyName string) (SigningKeyFactory, error) {
	if client == nil {
		return nil, fmt.Errorf("Google Cloud KMS client is required")
	}
	if cryptoKeyName == "" {
		return nil, fmt.Errorf("Google Cloud KMS signing CryptoKey is required")
	}
	return func(ctx context.Context, status string, notBefore, notAfter *time.Time, alg string) (*model.JWTSigningKey, error) {
		if alg != AlgorithmES256 {
			return nil, fmt.Errorf("unsupported JWT signing algorithm for Google Cloud KMS: %s", alg)
		}
		version, err := client.CreateCryptoKeyVersion(ctx, &kmspb.CreateCryptoKeyVersionRequest{
			Parent:           cryptoKeyName,
			CryptoKeyVersion: &kmspb.CryptoKeyVersion{},
		})
		if err != nil {
			return nil, fmt.Errorf("create Google Cloud KMS signing key version: %w", err)
		}
		key, err := NewGCPKMSSigningKey(ctx, client, version.Name)
		if err != nil {
			return nil, err
		}
		key.Status = status
		key.NotBefore = notBefore
		key.NotAfter = notAfter
		return key, nil
	}, nil
}

func (s *GCPKMSSigner) Sign(ctx context.Context, key *model.JWTSigningKey, signingInput []byte) ([]byte, error) {
	if key.Backend != BackendGCPKMS {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSignerBackend, key.Backend)
	}
	if key.Algorithm != AlgorithmES256 {
		return nil, fmt.Errorf("unsupported JWT signing algorithm for Google Cloud KMS: %s", key.Algorithm)
	}
	if key.BackendKeyID == nil || *key.BackendKeyID == "" {
		return nil, fmt.Errorf("Google Cloud KMS signing key version is required")
	}
	digest := sha256.Sum256(signingInput)
	resp, err := s.client.AsymmetricSign(ctx, &kmspb.AsymmetricSignRequest{
		Name: *key.BackendKeyID,
		Digest: &kmspb.Digest{
			Digest: &kmspb.Digest_Sha256{Sha256: digest[:]},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("sign JWT with Google Cloud KMS: %w", err)
	}
	return es256SignatureFromDER(resp.Signature)
}

// NewGCPKMSSigningKey builds a Meta signing-key row from a KMS key version.
func NewGCPKMSSigningKey(ctx context.Context, client gcpKMSClient, keyVersionName string) (*model.JWTSigningKey, error) {
	if keyVersionName == "" {
		return nil, fmt.Errorf("Google Cloud KMS key version is required")
	}
	if client == nil {
		return nil, fmt.Errorf("Google Cloud KMS client is required")
	}
	pub, err := client.GetPublicKey(ctx, &kmspb.GetPublicKeyRequest{Name: keyVersionName})
	if err != nil {
		return nil, fmt.Errorf("get Google Cloud KMS public key: %w", err)
	}
	if pub.Algorithm != kmspb.CryptoKeyVersion_EC_SIGN_P256_SHA256 {
		return nil, fmt.Errorf("Google Cloud KMS key version must use EC_SIGN_P256_SHA256, got %s", pub.Algorithm.String())
	}
	publicKey, err := parseGCPKMSPublicKey(pub.Pem)
	if err != nil {
		return nil, err
	}
	rowID := id.MustNew(id.TypeJWTSigningKey)
	keyID := id.MustNew(id.TypeJWTSigningKey)
	publicJWK, err := publicJWK(keyID, *publicKey)
	if err != nil {
		return nil, err
	}
	return &model.JWTSigningKey{
		ID:            rowID,
		KeyID:         keyID,
		Algorithm:     AlgorithmES256,
		Backend:       BackendGCPKMS,
		BackendKeyID:  &keyVersionName,
		PublicJWKJSON: publicJWK,
		Status:        model.JWTSigningKeyStatusNext,
	}, nil
}

func parseGCPKMSPublicKey(publicKeyPEM string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, errors.New("decode Google Cloud KMS public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse Google Cloud KMS public key: %w", err)
	}
	publicKey, ok := parsed.(*ecdsa.PublicKey)
	if !ok || publicKey.Curve != elliptic.P256() {
		return nil, errors.New("Google Cloud KMS public key must be ECDSA P-256")
	}
	return publicKey, nil
}

func es256SignatureFromDER(signature []byte) ([]byte, error) {
	var parsed struct {
		R *big.Int
		S *big.Int
	}
	rest, err := asn1.Unmarshal(signature, &parsed)
	if err != nil {
		return nil, fmt.Errorf("parse Google Cloud KMS ECDSA signature: %w", err)
	}
	if len(rest) != 0 || parsed.R == nil || parsed.S == nil {
		return nil, errors.New("invalid Google Cloud KMS ECDSA signature")
	}
	return es256Signature(parsed.R, parsed.S), nil
}
