package jwtkeys

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"

	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/googleapis/gax-go/v2"

	"github.com/obot-platform/discobot/meta/internal/model"
)

func TestGCPKMSSignerSignsWithKMSKeyVersion(t *testing.T) {
	client := newFakeGCPJWTKMSClient(t)
	key, err := NewGCPKMSSigningKey(context.Background(), client, client.keyVersionName)
	if err != nil {
		t.Fatal(err)
	}
	if key.Backend != BackendGCPKMS || key.BackendKeyID == nil || *key.BackendKeyID != client.keyVersionName {
		t.Fatalf("unexpected key metadata: %#v", key)
	}

	var jwk map[string]string
	if err := json.Unmarshal(key.PublicJWKJSON, &jwk); err != nil {
		t.Fatal(err)
	}
	if jwk["kid"] != key.KeyID || jwk["alg"] != AlgorithmES256 || jwk["use"] != "sig" {
		t.Fatalf("unexpected JWK metadata: %#v", jwk)
	}

	signingInput := []byte("eyJhbGciOiJFUzI1NiJ9.eyJzdWIiOiJ1c3JfMTIzIn0")
	signer, err := NewGCPKMSSignerWithClient(client)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := signer.Sign(context.Background(), key, signingInput)
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) != 64 {
		t.Fatalf("signature length = %d", len(sig))
	}
	if client.signName != client.keyVersionName {
		t.Fatalf("KMS sign name = %q", client.signName)
	}

	pub := publicKeyFromJWK(t, jwk)
	digest := sha256.Sum256(signingInput)
	if !ecdsa.Verify(pub, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Fatal("signature did not verify")
	}
}

func TestGCPKMSSignerRejectsMissingBackendKeyID(t *testing.T) {
	signer, err := NewGCPKMSSignerWithClient(newFakeGCPJWTKMSClient(t))
	if err != nil {
		t.Fatal(err)
	}
	_, err = signer.Sign(context.Background(), &model.JWTSigningKey{
		Backend:   BackendGCPKMS,
		Algorithm: AlgorithmES256,
	}, []byte("input"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewGCPKMSSigningKeyRejectsWrongAlgorithm(t *testing.T) {
	client := newFakeGCPJWTKMSClient(t)
	client.algorithm = kmspb.CryptoKeyVersion_RSA_SIGN_PSS_2048_SHA256
	_, err := NewGCPKMSSigningKey(context.Background(), client, client.keyVersionName)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGCPKMSSigningKeyFactoryCreatesKeyVersion(t *testing.T) {
	client := newFakeGCPJWTKMSClient(t)
	factory, err := NewGCPKMSSigningKeyFactoryWithClient(client, "projects/p/locations/global/keyRings/r/cryptoKeys/k")
	if err != nil {
		t.Fatal(err)
	}
	key, err := factory(context.Background(), model.JWTSigningKeyStatusActive, nil, nil, AlgorithmES256)
	if err != nil {
		t.Fatal(err)
	}
	if client.createParent != "projects/p/locations/global/keyRings/r/cryptoKeys/k" {
		t.Fatalf("create parent = %q", client.createParent)
	}
	if key.Backend != BackendGCPKMS || key.BackendKeyID == nil || *key.BackendKeyID != client.keyVersionName {
		t.Fatalf("unexpected key metadata: %#v", key)
	}
	if key.Status != model.JWTSigningKeyStatusActive {
		t.Fatalf("expected active key, got %q", key.Status)
	}
}

type fakeGCPJWTKMSClient struct {
	privateKey     *ecdsa.PrivateKey
	keyVersionName string
	algorithm      kmspb.CryptoKeyVersion_CryptoKeyVersionAlgorithm
	signName       string
	createParent   string
}

func newFakeGCPJWTKMSClient(t *testing.T) *fakeGCPJWTKMSClient {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeGCPJWTKMSClient{
		privateKey:     privateKey,
		keyVersionName: "projects/p/locations/global/keyRings/r/cryptoKeys/k/cryptoKeyVersions/1",
		algorithm:      kmspb.CryptoKeyVersion_EC_SIGN_P256_SHA256,
	}
}

func (f *fakeGCPJWTKMSClient) AsymmetricSign(_ context.Context, req *kmspb.AsymmetricSignRequest, _ ...gax.CallOption) (*kmspb.AsymmetricSignResponse, error) {
	f.signName = req.Name
	digest := req.GetDigest().GetSha256()
	r, s, err := ecdsa.Sign(rand.Reader, f.privateKey, digest)
	if err != nil {
		return nil, err
	}
	sig, err := asn1.Marshal(struct {
		R *big.Int
		S *big.Int
	}{R: r, S: s})
	if err != nil {
		return nil, err
	}
	return &kmspb.AsymmetricSignResponse{Signature: sig}, nil
}

func (f *fakeGCPJWTKMSClient) CreateCryptoKeyVersion(_ context.Context, req *kmspb.CreateCryptoKeyVersionRequest, _ ...gax.CallOption) (*kmspb.CryptoKeyVersion, error) {
	f.createParent = req.Parent
	return &kmspb.CryptoKeyVersion{
		Name:      f.keyVersionName,
		Algorithm: f.algorithm,
	}, nil
}

func (f *fakeGCPJWTKMSClient) GetPublicKey(_ context.Context, req *kmspb.GetPublicKeyRequest, _ ...gax.CallOption) (*kmspb.PublicKey, error) {
	der, err := x509.MarshalPKIXPublicKey(&f.privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	return &kmspb.PublicKey{
		Name:      req.Name,
		Pem:       string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})),
		Algorithm: f.algorithm,
	}, nil
}
