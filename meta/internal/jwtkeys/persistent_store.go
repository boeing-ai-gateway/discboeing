package jwtkeys

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/model"
)

type signingKeyDataStore interface {
	CreateJWTSigningKey(ctx context.Context, key *model.JWTSigningKey) error
	ListJWTSigningKeys(ctx context.Context, organizationID *string) ([]*model.JWTSigningKey, error)
	UpdateJWTSigningKeyStatus(ctx context.Context, id, status string, notBefore, notAfter *time.Time) error
}

// SigningKeyFactory creates new provider-backed JWT signing keys.
type SigningKeyFactory func(ctx context.Context, status string, notBefore, notAfter *time.Time, alg string) (*model.JWTSigningKey, error)

// PersistentSigningKeyStore stores Meta-owned JWT signing keys in the database.
type PersistentSigningKeyStore struct {
	store          signingKeyDataStore
	encryptor      dbcrypt.Encryptor
	backend        string
	algorithm      string
	organizationID *string
	policy         RotationPolicy
	autoCreateKeys bool
	keyFactory     SigningKeyFactory
	now            func() time.Time
}

// PersistentSigningKeyStoreOptions configures a persistent signing-key store.
type PersistentSigningKeyStoreOptions struct {
	Backend        string
	Algorithm      string
	OrganizationID *string
	Policy         RotationPolicy
	AutoCreateKeys bool
	KeyFactory     SigningKeyFactory
}

// NewPersistentSigningKeyStore returns a DB-backed signing-key store.
func NewPersistentSigningKeyStore(store signingKeyDataStore, encryptor dbcrypt.Encryptor, opts PersistentSigningKeyStoreOptions) *PersistentSigningKeyStore {
	if opts.Backend == "" {
		opts.Backend = BackendDBLocal
	}
	if opts.Algorithm == "" {
		opts.Algorithm = AlgorithmES256
	}
	keyFactory := opts.KeyFactory
	if keyFactory == nil && opts.Backend == BackendDBLocal {
		keyFactory = func(ctx context.Context, status string, notBefore, notAfter *time.Time, alg string) (*model.JWTSigningKey, error) {
			if alg != AlgorithmES256 {
				return nil, fmt.Errorf("unsupported JWT signing algorithm: %s", alg)
			}
			key, err := NewDBLocalSigningKey(ctx, encryptor)
			if err != nil {
				return nil, err
			}
			key.Status = status
			key.NotBefore = notBefore
			key.NotAfter = notAfter
			return key, nil
		}
	}
	autoCreateKeys := opts.AutoCreateKeys || keyFactory != nil
	return &PersistentSigningKeyStore{
		store:          store,
		encryptor:      encryptor,
		backend:        opts.Backend,
		algorithm:      opts.Algorithm,
		organizationID: opts.OrganizationID,
		policy:         opts.Policy.normalized(),
		autoCreateKeys: autoCreateKeys,
		keyFactory:     keyFactory,
		now:            func() time.Time { return time.Now().UTC() },
	}
}

// ActiveSigningKey returns the current persisted key for signing new JWTs.
func (s *PersistentSigningKeyStore) ActiveSigningKey(ctx context.Context, _ string) (*model.JWTSigningKey, error) {
	keys, err := s.store.ListJWTSigningKeys(ctx, s.organizationID)
	if err != nil {
		return nil, err
	}
	key := ActiveSigningKeyAt(s.now(), keys)
	if key == nil {
		return nil, ErrNoActiveSigningKey
	}
	return key, nil
}

// PublicJWKS returns persisted public verification keys for the issuer.
func (s *PersistentSigningKeyStore) PublicJWKS(ctx context.Context, _ string) (json.RawMessage, error) {
	keys, err := s.store.ListJWTSigningKeys(ctx, s.organizationID)
	if err != nil {
		return nil, err
	}
	return PublicJWKS(s.now(), keys)
}

// CreateNextKey creates a persisted next key for this store's backend.
func (s *PersistentSigningKeyStore) CreateNextKey(ctx context.Context, _ string, alg string) (*model.JWTSigningKey, error) {
	if alg == "" {
		alg = s.algorithm
	}
	key, err := s.newSigningKey(ctx, model.JWTSigningKeyStatusNext, nil, nil, alg)
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateJWTSigningKey(ctx, key); err != nil {
		return nil, err
	}
	return key, nil
}

// PromoteKey makes one persisted key active.
func (s *PersistentSigningKeyStore) PromoteKey(ctx context.Context, keyID string) error {
	notBefore := s.now()
	return s.store.UpdateJWTSigningKeyStatus(ctx, keyID, model.JWTSigningKeyStatusActive, &notBefore, nil)
}

// RetireKey marks one persisted key retired and keeps it in JWKS for the overlap.
func (s *PersistentSigningKeyStore) RetireKey(ctx context.Context, keyID string) error {
	retireUntil := s.now().Add(s.policy.VerificationOverlap)
	return s.RetireKeyUntil(ctx, keyID, retireUntil)
}

// RetireKeyUntil marks one persisted key retired until the provided time.
func (s *PersistentSigningKeyStore) RetireKeyUntil(ctx context.Context, keyID string, until time.Time) error {
	return s.store.UpdateJWTSigningKeyStatus(ctx, keyID, model.JWTSigningKeyStatusRetired, nil, &until)
}

// DisableKey removes one persisted key from signing and JWKS publication.
func (s *PersistentSigningKeyStore) DisableKey(ctx context.Context, keyID string) error {
	now := s.now()
	return s.store.UpdateJWTSigningKeyStatus(ctx, keyID, model.JWTSigningKeyStatusDisabled, nil, &now)
}

// EnsureReady bootstraps and applies one rotation step for this issuer.
func (s *PersistentSigningKeyStore) EnsureReady(ctx context.Context) error {
	keys, err := s.store.ListJWTSigningKeys(ctx, s.organizationID)
	if err != nil {
		return err
	}
	now := s.now()
	if len(keys) == 0 {
		if !s.autoCreateKeys {
			return nil
		}
		key, err := s.newSigningKey(ctx, model.JWTSigningKeyStatusActive, &now, nil, s.algorithm)
		if err != nil {
			return err
		}
		return s.store.CreateJWTSigningKey(ctx, key)
	}

	plan := PlanRotation(now, keys, s.policy)
	if plan.CreateNext {
		if !s.autoCreateKeys {
			return nil
		}
		key, err := s.newSigningKey(ctx, model.JWTSigningKeyStatusNext, nil, nil, s.algorithm)
		if err != nil {
			return err
		}
		return s.store.CreateJWTSigningKey(ctx, key)
	}
	if plan.PromoteKeyID != "" {
		if err := s.store.UpdateJWTSigningKeyStatus(ctx, plan.PromoteKeyID, model.JWTSigningKeyStatusActive, &now, nil); err != nil {
			return err
		}
	}
	if plan.RetireKeyID != "" {
		if err := s.store.UpdateJWTSigningKeyStatus(ctx, plan.RetireKeyID, model.JWTSigningKeyStatusRetired, nil, plan.RetireUntil); err != nil {
			return err
		}
	}
	return nil
}

func (s *PersistentSigningKeyStore) newSigningKey(ctx context.Context, status string, notBefore, notAfter *time.Time, alg string) (*model.JWTSigningKey, error) {
	if s.keyFactory == nil {
		return nil, fmt.Errorf("automatic signing key creation is not implemented for backend %s", s.backend)
	}
	key, err := s.keyFactory(ctx, status, notBefore, notAfter, alg)
	if err != nil {
		return nil, err
	}
	key.OrganizationID = s.organizationID
	return key, nil
}
