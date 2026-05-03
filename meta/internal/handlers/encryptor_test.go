package handlers

import (
	"testing"

	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
)

func newOAuthApplicationTestEncryptor(t *testing.T) dbcrypt.Encryptor {
	t.Helper()
	encryptor, err := dbcrypt.NewLocalEncryptor("test", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("new local encryptor: %v", err)
	}
	return encryptor
}
