package dbcrypt

import (
	"context"
	"errors"
	"testing"
)

func TestLocalEncryptorRequiresMatchingAAD(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	enc, err := NewLocalEncryptor("test", key)
	if err != nil {
		t.Fatal(err)
	}
	ref := FieldRef{Table: "jwt_signing_keys", Column: "private_key_encrypted", RowID: "jwk_123", Purpose: "jwt_signing_private_key"}
	envelope, err := enc.Encrypt(context.Background(), ref, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	plaintext, err := enc.Decrypt(context.Background(), ref, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != "secret" {
		t.Fatalf("plaintext = %q", plaintext)
	}

	ref.RowID = "jwk_other"
	_, err = enc.Decrypt(context.Background(), ref, envelope)
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("expected ErrAuthenticationFailed, got %v", err)
	}
}

func TestLocalEncryptorRejectsIncompleteFieldRef(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	enc, err := NewLocalEncryptor("test", key)
	if err != nil {
		t.Fatal(err)
	}
	_, err = enc.Encrypt(context.Background(), FieldRef{Table: "jwt_signing_keys"}, []byte("secret"))
	if !errors.Is(err, ErrInvalidFieldRef) {
		t.Fatalf("expected ErrInvalidFieldRef, got %v", err)
	}
}
