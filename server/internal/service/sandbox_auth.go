package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"aidanwoods.dev/go-paseto"

	"github.com/obot-platform/discobot/server/internal/encryption"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/store"
)

func encodeSandboxKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

func decodeSandboxPrivateKey(key string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid sandbox private key length")
	}
	return ed25519.PrivateKey(raw), nil
}

func createSandboxAuthToken(privateKey ed25519.PrivateKey, ttl time.Duration) (string, error) {
	pasetoKey, err := paseto.NewV4AsymmetricSecretKeyFromEd25519(privateKey)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	token := paseto.NewToken()
	now := time.Now()
	token.SetIssuedAt(now)
	token.SetNotBefore(now.Add(-time.Second))
	token.SetExpiration(now.Add(ttl))
	token.SetJti(encodeSandboxKey(nonce))
	return token.V4Sign(pasetoKey, nil), nil
}

func generateUserSandboxKeyPair() (publicKey, privateKey string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return encodeSandboxKey(pub), encodeSandboxKey(priv), nil
}

func ensureSandboxKeysForUser(ctx context.Context, store *store.Store, encryptor *encryption.Encryptor, user *model.User) (string, error) {
	if user.SandboxPublicKey != "" && user.EncryptedSandboxPrivateKey != "" {
		return user.SandboxPublicKey, nil
	}

	publicKey, privateKey, err := generateUserSandboxKeyPair()
	if err != nil {
		return "", fmt.Errorf("failed to generate sandbox user key: %w", err)
	}
	encryptedPrivateKey, err := encryptor.Encrypt([]byte(privateKey))
	if err != nil {
		return "", fmt.Errorf("failed to encrypt sandbox user key: %w", err)
	}
	encodedPrivateKey := encodeSandboxKey(encryptedPrivateKey)
	updated, err := store.UpdateUserSandboxKeysIfMissing(ctx, user.ID, publicKey, encodedPrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to store sandbox user key: %w", err)
	}
	if updated {
		user.SandboxPublicKey = publicKey
		user.EncryptedSandboxPrivateKey = encodedPrivateKey
		return publicKey, nil
	}

	// Another request created keys first. Use the committed key pair so every
	// sandbox launched for this user trusts the private key now stored in the DB.
	reloaded, err := store.GetUserByID(ctx, user.ID)
	if err != nil {
		return "", fmt.Errorf("failed to reload sandbox user key: %w", err)
	}
	if reloaded.SandboxPublicKey == "" || reloaded.EncryptedSandboxPrivateKey == "" {
		return "", fmt.Errorf("sandbox user key was not stored")
	}
	user.SandboxPublicKey = reloaded.SandboxPublicKey
	user.EncryptedSandboxPrivateKey = reloaded.EncryptedSandboxPrivateKey
	return user.SandboxPublicKey, nil
}

func (s *SandboxService) ensureUserSandboxKeys(ctx context.Context, userID string) (publicKey string, err error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	encryptor, err := encryption.NewEncryptor(s.cfg.EncryptionKey)
	if err != nil {
		return "", err
	}
	return ensureSandboxKeysForUser(ctx, s.store, encryptor, user)
}

func (s *SandboxService) createSandboxAuthToken(ctx context.Context, sessionID string) (string, error) {
	session, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if session.CreatedByUserID == nil || *session.CreatedByUserID == "" {
		return "", nil
	}
	user, err := s.store.GetUserByID(ctx, *session.CreatedByUserID)
	if err != nil {
		return "", err
	}
	if user.EncryptedSandboxPrivateKey == "" {
		return "", nil
	}
	encryptedPrivateKey, err := base64.StdEncoding.DecodeString(user.EncryptedSandboxPrivateKey)
	if err != nil {
		return "", err
	}
	encryptor, err := encryption.NewEncryptor(s.cfg.EncryptionKey)
	if err != nil {
		return "", err
	}
	privateKeyText, err := encryptor.Decrypt(encryptedPrivateKey)
	if err != nil {
		return "", err
	}
	privateKey, err := decodeSandboxPrivateKey(string(privateKeyText))
	if err != nil {
		return "", err
	}
	return createSandboxAuthToken(privateKey, time.Minute)
}

func (s *SandboxService) sandboxTrustKeyForUser(ctx context.Context, userID *string) (string, error) {
	if userID == nil || *userID == "" {
		return "", nil
	}
	return s.ensureUserSandboxKeys(ctx, *userID)
}
