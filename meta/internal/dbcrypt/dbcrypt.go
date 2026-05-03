package dbcrypt

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	ProviderLocal      = "local"
	ProviderGCPKMS     = "gcp-kms"
	AlgorithmAES256GCM = "AES-256-GCM"
	ServiceMeta        = "meta"
)

var (
	ErrAuthenticationFailed = errors.New("database encrypted field authentication failed")
	ErrInvalidFieldRef      = errors.New("invalid encrypted field reference")
)

// Encryptor encrypts and decrypts database fields with mandatory AAD.
type Encryptor interface {
	Encrypt(ctx context.Context, ref FieldRef, plaintext []byte) (Envelope, error)
	Decrypt(ctx context.Context, ref FieldRef, envelope Envelope) ([]byte, error)
}

// FieldRef identifies one encrypted field on one row. It is the source of AAD.
type FieldRef struct {
	Table   string `json:"table"`
	Column  string `json:"column"`
	RowID   string `json:"row_id"`
	Purpose string `json:"purpose"`
}

// Envelope is the provider-neutral encrypted database field envelope.
type Envelope struct {
	Version      int       `json:"version"`
	Provider     string    `json:"provider"`
	KeyID        string    `json:"key_id"`
	Algorithm    string    `json:"algorithm"`
	WrapAlg      string    `json:"wrap_algorithm"`
	Nonce        string    `json:"nonce"`
	Ciphertext   string    `json:"ciphertext"`
	EncryptedDEK string    `json:"encrypted_dek"`
	AAD          AAD       `json:"aad"`
	CreatedAt    time.Time `json:"created_at"`
}

// AAD is stored for diagnostics. Decrypt still uses the caller-provided FieldRef.
type AAD struct {
	Version int    `json:"version"`
	Service string `json:"service"`
	Table   string `json:"table"`
	Column  string `json:"column"`
	RowID   string `json:"row_id"`
	Purpose string `json:"purpose"`
}

// LocalEncryptor uses a local 256-bit key to wrap per-field DEKs.
type LocalEncryptor struct {
	keyID string
	key   []byte
}

// NewLocalEncryptor returns an Encryptor backed by a local 256-bit wrapping key.
func NewLocalEncryptor(keyID string, key []byte) (*LocalEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("local database encryption key must be 32 bytes, got %d", len(key))
	}
	if keyID == "" {
		keyID = ProviderLocal
	}
	return &LocalEncryptor{keyID: keyID, key: append([]byte(nil), key...)}, nil
}

func (e *LocalEncryptor) Encrypt(_ context.Context, ref FieldRef, plaintext []byte) (Envelope, error) {
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
	encryptedDEK, dekNonce, err := seal(e.key, dek, aadBytes)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		Version:      1,
		Provider:     ProviderLocal,
		KeyID:        e.keyID,
		Algorithm:    AlgorithmAES256GCM,
		WrapAlg:      "LOCAL-AES-256-GCM",
		Nonce:        encode(nonce),
		Ciphertext:   encode(ciphertext),
		EncryptedDEK: encode(append(dekNonce, encryptedDEK...)),
		AAD:          aad,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

func (e *LocalEncryptor) Decrypt(_ context.Context, ref FieldRef, envelope Envelope) ([]byte, error) {
	aad, aadBytes, err := canonicalAAD(ref)
	if err != nil {
		return nil, err
	}
	if envelope.AAD != aad {
		return nil, ErrAuthenticationFailed
	}
	if envelope.Provider != ProviderLocal || envelope.Algorithm != AlgorithmAES256GCM {
		return nil, fmt.Errorf("unsupported encrypted field envelope provider=%q algorithm=%q", envelope.Provider, envelope.Algorithm)
	}

	wrapped, err := decode(envelope.EncryptedDEK)
	if err != nil {
		return nil, err
	}
	if len(wrapped) < 12 {
		return nil, ErrAuthenticationFailed
	}
	dekNonce, encryptedDEK := wrapped[:12], wrapped[12:]
	dek, err := open(e.key, dekNonce, encryptedDEK, aadBytes)
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
	plaintext, err := open(dek, nonce, ciphertext, aadBytes)
	if err != nil {
		return nil, ErrAuthenticationFailed
	}
	return plaintext, nil
}

func canonicalAAD(ref FieldRef) (AAD, []byte, error) {
	if ref.Table == "" || ref.Column == "" || ref.RowID == "" || ref.Purpose == "" {
		return AAD{}, nil, ErrInvalidFieldRef
	}
	aad := AAD{Version: 1, Service: ServiceMeta, Table: ref.Table, Column: ref.Column, RowID: ref.RowID, Purpose: ref.Purpose}
	data, err := json.Marshal(aad)
	if err != nil {
		return AAD{}, nil, err
	}
	return aad, data, nil
}

func seal(key, plaintext, aad []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, aad), nonce, nil
}

func open(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func encode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func decode(value string) ([]byte, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted field envelope: %w", err)
	}
	return data, nil
}

// Equal reports whether two envelopes are byte-for-byte equivalent as JSON.
func Equal(a, b Envelope) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return bytes.Equal(aj, bj)
}
