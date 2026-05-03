package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/model"
)

// FieldID identifies one supported encrypted database field.
type FieldID string

const (
	// FieldJWTSigningPrivateKey encrypts JWTSigningKey.PrivateKeyEncrypted.
	FieldJWTSigningPrivateKey FieldID = "jwt_signing_key.private_key"
	// FieldOAuthApplicationClientSecret encrypts OAuthApplication.ClientSecretEncrypted.
	FieldOAuthApplicationClientSecret FieldID = "oauth_application.client_secret"
)

var (
	ErrInvalidEncryptedField     = errors.New("invalid encrypted field for row")
	ErrUnsupportedEncryptedField = errors.New("unsupported encrypted field")
	ErrMissingEncryptor          = errors.New("database encryptor is required")
	ErrMissingRowID              = errors.New("encrypted row ID is required")
	ErrMissingEncryptedValue     = errors.New("encrypted field value is empty")
)

type encryptedFieldSpec struct {
	column    string
	purpose   string
	fieldRef  func(row any) (dbcrypt.FieldRef, error)
	getCipher func(row any) ([]byte, error)
	setCipher func(row any, ciphertext []byte) error
}

var encryptedFieldSpecs = map[FieldID]encryptedFieldSpec{
	FieldOAuthApplicationClientSecret: {
		column:  "client_secret_encrypted",
		purpose: "oauth_client_secret",
		fieldRef: func(row any) (dbcrypt.FieldRef, error) {
			app, ok := row.(*model.OAuthApplication)
			if !ok {
				return dbcrypt.FieldRef{}, ErrInvalidEncryptedField
			}
			if app.ID == "" {
				return dbcrypt.FieldRef{}, ErrMissingRowID
			}
			return dbcrypt.FieldRef{Table: app.TableName(), Column: "client_secret_encrypted", RowID: app.ID, Purpose: "oauth_client_secret"}, nil
		},
		getCipher: func(row any) ([]byte, error) {
			app, ok := row.(*model.OAuthApplication)
			if !ok {
				return nil, ErrInvalidEncryptedField
			}
			return app.ClientSecretEncrypted, nil
		},
		setCipher: func(row any, ciphertext []byte) error {
			app, ok := row.(*model.OAuthApplication)
			if !ok {
				return ErrInvalidEncryptedField
			}
			app.ClientSecretEncrypted = ciphertext
			return nil
		},
	},
	FieldJWTSigningPrivateKey: {
		column:  "private_key_encrypted",
		purpose: "jwt_signing_private_key",
		fieldRef: func(row any) (dbcrypt.FieldRef, error) {
			key, ok := row.(*model.JWTSigningKey)
			if !ok {
				return dbcrypt.FieldRef{}, ErrInvalidEncryptedField
			}
			if key.ID == "" {
				return dbcrypt.FieldRef{}, ErrMissingRowID
			}
			return dbcrypt.FieldRef{Table: key.TableName(), Column: "private_key_encrypted", RowID: key.ID, Purpose: "jwt_signing_private_key"}, nil
		},
		getCipher: func(row any) ([]byte, error) {
			key, ok := row.(*model.JWTSigningKey)
			if !ok {
				return nil, ErrInvalidEncryptedField
			}
			return key.PrivateKeyEncrypted, nil
		},
		setCipher: func(row any, ciphertext []byte) error {
			key, ok := row.(*model.JWTSigningKey)
			if !ok {
				return ErrInvalidEncryptedField
			}
			key.PrivateKeyEncrypted = ciphertext
			return nil
		},
	},
}

// SetEncryptedField encrypts cleartext and stores the envelope on row's field.
func SetEncryptedField(ctx context.Context, encryptor dbcrypt.Encryptor, row any, fieldID FieldID, cleartext []byte) error {
	if encryptor == nil {
		return ErrMissingEncryptor
	}
	spec, err := encryptedFieldSpecFor(fieldID)
	if err != nil {
		return err
	}
	ref, err := spec.fieldRef(row)
	if err != nil {
		return err
	}
	envelope, err := encryptor.Encrypt(ctx, ref, cleartext)
	if err != nil {
		return err
	}
	ciphertext, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return spec.setCipher(row, ciphertext)
}

// DecryptField decrypts and returns cleartext from row's encrypted field.
func DecryptField(ctx context.Context, encryptor dbcrypt.Encryptor, row any, fieldID FieldID) ([]byte, error) {
	if encryptor == nil {
		return nil, ErrMissingEncryptor
	}
	spec, err := encryptedFieldSpecFor(fieldID)
	if err != nil {
		return nil, err
	}
	ciphertext, err := spec.getCipher(row)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) == 0 {
		return nil, ErrMissingEncryptedValue
	}
	var envelope dbcrypt.Envelope
	if err := json.Unmarshal(ciphertext, &envelope); err != nil {
		return nil, err
	}
	ref, err := spec.fieldRef(row)
	if err != nil {
		return nil, err
	}
	return encryptor.Decrypt(ctx, ref, envelope)
}

func encryptedFieldSpecFor(fieldID FieldID) (encryptedFieldSpec, error) {
	spec, ok := encryptedFieldSpecs[fieldID]
	if !ok {
		return encryptedFieldSpec{}, ErrUnsupportedEncryptedField
	}
	return spec, nil
}
