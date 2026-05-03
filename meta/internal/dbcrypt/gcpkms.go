package dbcrypt

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"time"

	cloudkms "cloud.google.com/go/kms/apiv1"
	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/googleapis/gax-go/v2"
)

const gcpKMSWrapAlg = "GCP-CLOUD-KMS"

type gcpKMSClient interface {
	Encrypt(context.Context, *kmspb.EncryptRequest, ...gax.CallOption) (*kmspb.EncryptResponse, error)
	Decrypt(context.Context, *kmspb.DecryptRequest, ...gax.CallOption) (*kmspb.DecryptResponse, error)
}

// GCPKMSEncryptor uses Google Cloud KMS to wrap per-field DEKs.
type GCPKMSEncryptor struct {
	keyName string
	client  gcpKMSClient
}

// NewGCPKMSEncryptor returns an Encryptor backed by a Google Cloud KMS key.
func NewGCPKMSEncryptor(ctx context.Context, keyName string) (*GCPKMSEncryptor, error) {
	client, err := cloudkms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create Google Cloud KMS client: %w", err)
	}
	return NewGCPKMSEncryptorWithClient(keyName, client)
}

// NewGCPKMSEncryptorWithClient returns a GCP KMS encryptor using an existing client.
func NewGCPKMSEncryptorWithClient(keyName string, client gcpKMSClient) (*GCPKMSEncryptor, error) {
	if keyName == "" {
		return nil, fmt.Errorf("Google Cloud KMS key name is required")
	}
	if client == nil {
		return nil, fmt.Errorf("Google Cloud KMS client is required")
	}
	return &GCPKMSEncryptor{keyName: keyName, client: client}, nil
}

func (e *GCPKMSEncryptor) Encrypt(ctx context.Context, ref FieldRef, plaintext []byte) (Envelope, error) {
	aad, aadBytes, err := canonicalAAD(ref)
	if err != nil {
		return Envelope{}, err
	}

	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return Envelope{}, fmt.Errorf("generate DEK: %w", err)
	}

	ciphertext, nonce, err := seal(dek, plaintext, aadBytes)
	if err != nil {
		return Envelope{}, err
	}
	wrapped, err := e.client.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:                        e.keyName,
		Plaintext:                   dek,
		AdditionalAuthenticatedData: aadBytes,
	})
	if err != nil {
		return Envelope{}, fmt.Errorf("wrap DEK with Google Cloud KMS: %w", err)
	}
	return Envelope{
		Version:      1,
		Provider:     ProviderGCPKMS,
		KeyID:        e.keyName,
		Algorithm:    AlgorithmAES256GCM,
		WrapAlg:      gcpKMSWrapAlg,
		Nonce:        encode(nonce),
		Ciphertext:   encode(ciphertext),
		EncryptedDEK: encode(wrapped.Ciphertext),
		AAD:          aad,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

func (e *GCPKMSEncryptor) Decrypt(ctx context.Context, ref FieldRef, envelope Envelope) ([]byte, error) {
	aad, aadBytes, err := canonicalAAD(ref)
	if err != nil {
		return nil, err
	}
	if envelope.AAD != aad {
		return nil, ErrAuthenticationFailed
	}
	if envelope.Provider != ProviderGCPKMS || envelope.Algorithm != AlgorithmAES256GCM || envelope.WrapAlg != gcpKMSWrapAlg {
		return nil, fmt.Errorf("unsupported encrypted field envelope provider=%q algorithm=%q wrap_algorithm=%q", envelope.Provider, envelope.Algorithm, envelope.WrapAlg)
	}
	if envelope.KeyID == "" {
		return nil, fmt.Errorf("encrypted field envelope is missing Google Cloud KMS key name")
	}

	wrapped, err := decode(envelope.EncryptedDEK)
	if err != nil {
		return nil, err
	}
	dek, err := e.client.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:                        envelope.KeyID,
		Ciphertext:                  wrapped,
		AdditionalAuthenticatedData: aadBytes,
	})
	if err != nil {
		return nil, ErrAuthenticationFailed
	}

	nonce, err := decode(envelope.Nonce)
	if err != nil {
		return nil, err
	}
	ciphertext, err := decode(envelope.Ciphertext)
	if err != nil {
		return nil, err
	}
	plaintext, err := open(dek.Plaintext, nonce, ciphertext, aadBytes)
	if err != nil {
		return nil, ErrAuthenticationFailed
	}
	return plaintext, nil
}
