package dbcrypt

import (
	"bytes"
	"context"
	"errors"
	"testing"

	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/googleapis/gax-go/v2"
)

func TestGCPKMSEncryptorRoundTrip(t *testing.T) {
	client := &fakeGCPKMSClient{}
	enc, err := NewGCPKMSEncryptorWithClient("projects/p/locations/global/keyRings/r/cryptoKeys/k", client)
	if err != nil {
		t.Fatal(err)
	}
	ref := FieldRef{Table: "jwt_signing_keys", Column: "private_key_encrypted", RowID: "jwk_123", Purpose: "jwt_signing_private_key"}
	envelope, err := enc.Encrypt(context.Background(), ref, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Provider != ProviderGCPKMS || envelope.KeyID != enc.keyName || envelope.WrapAlg != gcpKMSWrapAlg {
		t.Fatalf("unexpected envelope metadata: %#v", envelope)
	}
	if client.encryptName != enc.keyName || len(client.encryptAAD) == 0 {
		t.Fatalf("KMS encrypt request did not include key name and AAD")
	}

	plaintext, err := enc.Decrypt(context.Background(), ref, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != "secret" {
		t.Fatalf("plaintext = %q", plaintext)
	}
	if client.decryptName != enc.keyName || !bytes.Equal(client.decryptAAD, client.encryptAAD) {
		t.Fatalf("KMS decrypt request did not reuse key name and AAD")
	}
}

func TestGCPKMSEncryptorRejectsMismatchedAAD(t *testing.T) {
	enc, err := NewGCPKMSEncryptorWithClient("projects/p/locations/global/keyRings/r/cryptoKeys/k", &fakeGCPKMSClient{})
	if err != nil {
		t.Fatal(err)
	}
	ref := FieldRef{Table: "jwt_signing_keys", Column: "private_key_encrypted", RowID: "jwk_123", Purpose: "jwt_signing_private_key"}
	envelope, err := enc.Encrypt(context.Background(), ref, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	ref.RowID = "jwk_other"
	_, err = enc.Decrypt(context.Background(), ref, envelope)
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("expected ErrAuthenticationFailed, got %v", err)
	}
}

type fakeGCPKMSClient struct {
	encryptName string
	encryptAAD  []byte
	decryptName string
	decryptAAD  []byte
}

func (f *fakeGCPKMSClient) Encrypt(_ context.Context, req *kmspb.EncryptRequest, _ ...gax.CallOption) (*kmspb.EncryptResponse, error) {
	f.encryptName = req.Name
	f.encryptAAD = append([]byte(nil), req.AdditionalAuthenticatedData...)
	ciphertext := append([]byte(nil), req.AdditionalAuthenticatedData...)
	ciphertext = append(ciphertext, 0)
	ciphertext = append(ciphertext, req.Plaintext...)
	return &kmspb.EncryptResponse{Ciphertext: ciphertext}, nil
}

func (f *fakeGCPKMSClient) Decrypt(_ context.Context, req *kmspb.DecryptRequest, _ ...gax.CallOption) (*kmspb.DecryptResponse, error) {
	f.decryptName = req.Name
	f.decryptAAD = append([]byte(nil), req.AdditionalAuthenticatedData...)
	prefix := append([]byte(nil), req.AdditionalAuthenticatedData...)
	prefix = append(prefix, 0)
	if !bytes.HasPrefix(req.Ciphertext, prefix) {
		return nil, errors.New("AAD mismatch")
	}
	return &kmspb.DecryptResponse{Plaintext: append([]byte(nil), req.Ciphertext[len(prefix):]...)}, nil
}
