package store

import (
	"context"
	"errors"
	"testing"

	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/model"
)

func TestEncryptedFieldHelpersRoundTripOAuthClientSecret(t *testing.T) {
	enc := testLocalEncryptor(t)
	app := &model.OAuthApplication{ID: "oauth_app_123"}

	if err := SetEncryptedField(context.Background(), enc, app, FieldOAuthApplicationClientSecret, []byte("client-secret")); err != nil {
		t.Fatal(err)
	}
	if string(app.ClientSecretEncrypted) == "client-secret" {
		t.Fatal("encrypted field stored plaintext")
	}

	plaintext, err := DecryptField(context.Background(), enc, app, FieldOAuthApplicationClientSecret)
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != "client-secret" {
		t.Fatalf("plaintext = %q", plaintext)
	}
}

func TestEncryptedFieldHelpersRoundTripJWTSigningKey(t *testing.T) {
	enc := testLocalEncryptor(t)
	key := &model.JWTSigningKey{ID: "jwk_123"}

	if err := SetEncryptedField(context.Background(), enc, key, FieldJWTSigningPrivateKey, []byte("private-key")); err != nil {
		t.Fatal(err)
	}
	plaintext, err := DecryptField(context.Background(), enc, key, FieldJWTSigningPrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != "private-key" {
		t.Fatalf("plaintext = %q", plaintext)
	}
}

func TestEncryptedFieldHelpersRejectWrongDAO(t *testing.T) {
	enc := testLocalEncryptor(t)
	app := &model.OAuthApplication{ID: "oauth_app_123"}

	err := SetEncryptedField(context.Background(), enc, app, FieldJWTSigningPrivateKey, []byte("secret"))
	if !errors.Is(err, ErrInvalidEncryptedField) {
		t.Fatalf("expected ErrInvalidEncryptedField, got %v", err)
	}
}

func TestEncryptedFieldHelpersRejectUnsupportedField(t *testing.T) {
	enc := testLocalEncryptor(t)
	app := &model.OAuthApplication{ID: "oauth_app_123"}

	err := SetEncryptedField(context.Background(), enc, app, FieldID("unknown.field"), []byte("secret"))
	if !errors.Is(err, ErrUnsupportedEncryptedField) {
		t.Fatalf("expected ErrUnsupportedEncryptedField, got %v", err)
	}
}

func TestEncryptedFieldHelpersRequireRowID(t *testing.T) {
	enc := testLocalEncryptor(t)
	app := &model.OAuthApplication{}

	err := SetEncryptedField(context.Background(), enc, app, FieldOAuthApplicationClientSecret, []byte("secret"))
	if !errors.Is(err, ErrMissingRowID) {
		t.Fatalf("expected ErrMissingRowID, got %v", err)
	}
}

func TestEncryptedFieldHelpersAuthenticateRowID(t *testing.T) {
	enc := testLocalEncryptor(t)
	app := &model.OAuthApplication{ID: "oauth_app_123"}
	if err := SetEncryptedField(context.Background(), enc, app, FieldOAuthApplicationClientSecret, []byte("client-secret")); err != nil {
		t.Fatal(err)
	}

	app.ID = "oauth_app_other"
	_, err := DecryptField(context.Background(), enc, app, FieldOAuthApplicationClientSecret)
	if !errors.Is(err, dbcrypt.ErrAuthenticationFailed) {
		t.Fatalf("expected ErrAuthenticationFailed, got %v", err)
	}
}

func testLocalEncryptor(t *testing.T) dbcrypt.Encryptor {
	t.Helper()
	enc, err := dbcrypt.NewLocalEncryptor("test", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return enc
}
