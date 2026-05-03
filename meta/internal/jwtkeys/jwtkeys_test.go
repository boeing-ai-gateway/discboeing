package jwtkeys

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
)

func TestDBLocalSignerSignsWithEncryptedKey(t *testing.T) {
	encryptor, err := dbcrypt.NewLocalEncryptor("test", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := NewDBLocalSigningKey(context.Background(), encryptor)
	if err != nil {
		t.Fatal(err)
	}

	var jwk map[string]string
	if err := json.Unmarshal(key.PublicJWKJSON, &jwk); err != nil {
		t.Fatal(err)
	}
	if jwk["kid"] != key.KeyID || jwk["alg"] != AlgorithmES256 || jwk["use"] != "sig" {
		t.Fatalf("unexpected JWK metadata: %#v", jwk)
	}

	signingInput := []byte("eyJhbGciOiJFUzI1NiJ9.eyJzdWIiOiJ1c3JfMTIzIn0")
	sig, err := NewDBLocalSigner(encryptor).Sign(context.Background(), key, signingInput)
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) != 64 {
		t.Fatalf("signature length = %d", len(sig))
	}

	pub := publicKeyFromJWK(t, jwk)
	digest := sha256.Sum256(signingInput)
	if !ecdsa.Verify(pub, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Fatal("signature did not verify")
	}
}

func TestDBLocalSignerFailsWhenEncryptedKeyAADDoesNotMatch(t *testing.T) {
	encryptor, err := dbcrypt.NewLocalEncryptor("test", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := NewDBLocalSigningKey(context.Background(), encryptor)
	if err != nil {
		t.Fatal(err)
	}
	key.ID = key.ID + "_moved"

	_, err = NewDBLocalSigner(encryptor).Sign(context.Background(), key, []byte("input"))
	if !errors.Is(err, dbcrypt.ErrAuthenticationFailed) {
		t.Fatalf("expected ErrAuthenticationFailed, got %v", err)
	}
}

func publicKeyFromJWK(t *testing.T, jwk map[string]string) *ecdsa.PublicKey {
	t.Helper()
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk["x"])
	if err != nil {
		t.Fatal(err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(jwk["y"])
	if err != nil {
		t.Fatal(err)
	}
	pub := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}
	if !pub.Curve.IsOnCurve(pub.X, pub.Y) {
		t.Fatal("JWK coordinates are not on P-256")
	}
	return pub
}
