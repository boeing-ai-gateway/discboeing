package jwtkeys

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/obot-platform/discobot/meta/internal/model"
)

const (
	DefaultRotationInterval    = 72 * time.Hour
	DefaultPrepublishWindow    = 24 * time.Hour
	DefaultVerificationOverlap = 7 * 24 * time.Hour
)

// RotationPolicy controls Meta-owned JWT signing key rollover.
type RotationPolicy struct {
	// Interval is the target maximum age for an active signing key.
	Interval time.Duration
	// PrepublishWindow is how long a next key should appear in JWKS before use.
	PrepublishWindow time.Duration
	// VerificationOverlap is how long retired keys remain in JWKS for verifiers.
	VerificationOverlap time.Duration
}

// DefaultRotationPolicy returns the default automatic rollover policy.
func DefaultRotationPolicy() RotationPolicy {
	return RotationPolicy{
		Interval:            DefaultRotationInterval,
		PrepublishWindow:    DefaultPrepublishWindow,
		VerificationOverlap: DefaultVerificationOverlap,
	}
}

func (p RotationPolicy) normalized() RotationPolicy {
	if p.Interval <= 0 {
		p.Interval = DefaultRotationInterval
	}
	if p.PrepublishWindow <= 0 {
		p.PrepublishWindow = DefaultPrepublishWindow
	}
	if p.VerificationOverlap <= 0 {
		p.VerificationOverlap = DefaultVerificationOverlap
	}
	return p
}

// RotationPlan describes non-destructive status changes a store should apply.
type RotationPlan struct {
	CreateNext   bool
	PromoteKeyID string
	RetireKeyID  string
	RetireUntil  *time.Time
}

// PlanRotation plans the next key lifecycle transition for one issuer.
func PlanRotation(now time.Time, keys []*model.JWTSigningKey, policy RotationPolicy) RotationPlan {
	policy = policy.normalized()
	active := activeSigningKeyAt(now, keys)
	next := nextSigningKeyAt(now, keys)
	if active == nil {
		if next != nil && readyForPromotion(now, next, policy) {
			return RotationPlan{PromoteKeyID: next.ID}
		}
		return RotationPlan{CreateNext: next == nil}
	}
	if next == nil && now.Sub(keyStart(active)) >= policy.Interval-policy.PrepublishWindow {
		return RotationPlan{CreateNext: true}
	}
	if next != nil && now.Sub(keyStart(active)) >= policy.Interval && readyForPromotion(now, next, policy) {
		retireUntil := now.Add(policy.VerificationOverlap)
		return RotationPlan{PromoteKeyID: next.ID, RetireKeyID: active.ID, RetireUntil: &retireUntil}
	}
	return RotationPlan{}
}

// ActiveSigningKeyAt returns the key that should sign new tokens at now.
func ActiveSigningKeyAt(now time.Time, keys []*model.JWTSigningKey) *model.JWTSigningKey {
	return activeSigningKeyAt(now, keys)
}

// PublicJWKS returns a JWKS containing active, next, and overlap-retired keys.
func PublicJWKS(now time.Time, keys []*model.JWTSigningKey) (json.RawMessage, error) {
	jwks := struct {
		Keys []json.RawMessage `json:"keys"`
	}{Keys: []json.RawMessage{}}
	for _, key := range keys {
		if !publishKeyAt(now, key) {
			continue
		}
		if !json.Valid(key.PublicJWKJSON) {
			return nil, fmt.Errorf("JWT signing key %s has invalid public JWK JSON", key.ID)
		}
		jwks.Keys = append(jwks.Keys, json.RawMessage(key.PublicJWKJSON))
	}
	return json.Marshal(jwks)
}

func activeSigningKeyAt(now time.Time, keys []*model.JWTSigningKey) *model.JWTSigningKey {
	var active *model.JWTSigningKey
	for _, key := range keys {
		if key == nil || key.Status != model.JWTSigningKeyStatusActive || !withinWindow(now, key) {
			continue
		}
		if active == nil || keyStart(key).After(keyStart(active)) {
			active = key
		}
	}
	return active
}

func nextSigningKeyAt(now time.Time, keys []*model.JWTSigningKey) *model.JWTSigningKey {
	var next *model.JWTSigningKey
	for _, key := range keys {
		if key == nil || key.Status != model.JWTSigningKeyStatusNext || expiredAt(now, key) {
			continue
		}
		if next == nil || keyStart(key).Before(keyStart(next)) {
			next = key
		}
	}
	return next
}

func publishKeyAt(now time.Time, key *model.JWTSigningKey) bool {
	if key == nil || len(key.PublicJWKJSON) == 0 || key.Status == model.JWTSigningKeyStatusDisabled {
		return false
	}
	switch key.Status {
	case model.JWTSigningKeyStatusNext, model.JWTSigningKeyStatusActive:
		return !expiredAt(now, key)
	case model.JWTSigningKeyStatusRetired:
		return key.NotAfter == nil || now.Before(*key.NotAfter)
	default:
		return false
	}
}

func readyForPromotion(now time.Time, key *model.JWTSigningKey, policy RotationPolicy) bool {
	if key.NotBefore != nil {
		return !now.Before(*key.NotBefore)
	}
	return now.Sub(key.CreatedAt) >= policy.PrepublishWindow
}

func keyStart(key *model.JWTSigningKey) time.Time {
	if key.NotBefore != nil {
		return *key.NotBefore
	}
	if !key.CreatedAt.IsZero() {
		return key.CreatedAt
	}
	return time.Time{}
}

func withinWindow(now time.Time, key *model.JWTSigningKey) bool {
	if key.NotBefore != nil && now.Before(*key.NotBefore) {
		return false
	}
	return !expiredAt(now, key)
}

func expiredAt(now time.Time, key *model.JWTSigningKey) bool {
	return key.NotAfter != nil && !now.Before(*key.NotAfter)
}
