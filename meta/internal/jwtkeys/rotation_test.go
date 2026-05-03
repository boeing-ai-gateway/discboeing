package jwtkeys

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/obot-platform/discobot/meta/internal/model"
)

func TestPlanRotationCreatesNextBeforeActiveExpires(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	activeCreated := now.Add(-49 * time.Hour)
	plan := PlanRotation(now, []*model.JWTSigningKey{{
		ID:        "active",
		Status:    model.JWTSigningKeyStatusActive,
		CreatedAt: activeCreated,
	}}, DefaultRotationPolicy())
	if !plan.CreateNext || plan.PromoteKeyID != "" || plan.RetireKeyID != "" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestPlanRotationPromotesPrepublishedNextKey(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	plan := PlanRotation(now, []*model.JWTSigningKey{
		{
			ID:        "active",
			Status:    model.JWTSigningKeyStatusActive,
			CreatedAt: now.Add(-73 * time.Hour),
		},
		{
			ID:        "next",
			Status:    model.JWTSigningKeyStatusNext,
			CreatedAt: now.Add(-25 * time.Hour),
		},
	}, DefaultRotationPolicy())
	if plan.CreateNext || plan.PromoteKeyID != "next" || plan.RetireKeyID != "active" || plan.RetireUntil == nil {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if got, want := plan.RetireUntil.Sub(now), DefaultVerificationOverlap; got != want {
		t.Fatalf("retire overlap = %s, want %s", got, want)
	}
}

func TestPublicJWKSIncludesOverlappingKeys(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	retiredUntil := now.Add(time.Hour)
	expiredRetiredUntil := now.Add(-time.Hour)
	jwksJSON, err := PublicJWKS(now, []*model.JWTSigningKey{
		jwksKey("next", model.JWTSigningKeyStatusNext, nil),
		jwksKey("active", model.JWTSigningKeyStatusActive, nil),
		jwksKey("retired", model.JWTSigningKeyStatusRetired, &retiredUntil),
		jwksKey("expired", model.JWTSigningKeyStatusRetired, &expiredRetiredUntil),
		jwksKey("disabled", model.JWTSigningKeyStatusDisabled, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	var jwks struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(jwksJSON, &jwks); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, key := range jwks.Keys {
		got[key["kid"]] = true
	}
	for _, kid := range []string{"next", "active", "retired"} {
		if !got[kid] {
			t.Fatalf("JWKS missing %q: %s", kid, jwksJSON)
		}
	}
	for _, kid := range []string{"expired", "disabled"} {
		if got[kid] {
			t.Fatalf("JWKS included %q: %s", kid, jwksJSON)
		}
	}
}

func TestActiveSigningKeyAtSkipsNotBefore(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	active := ActiveSigningKeyAt(now, []*model.JWTSigningKey{
		{ID: "current", Status: model.JWTSigningKeyStatusActive, CreatedAt: now.Add(-time.Hour)},
		{ID: "future", Status: model.JWTSigningKeyStatusActive, NotBefore: &future, CreatedAt: now.Add(-2 * time.Hour)},
	})
	if active == nil || active.ID != "current" {
		t.Fatalf("active key = %#v", active)
	}
}

func jwksKey(kid string, status string, notAfter *time.Time) *model.JWTSigningKey {
	return &model.JWTSigningKey{
		ID:            kid,
		KeyID:         kid,
		Status:        status,
		PublicJWKJSON: []byte(`{"kty":"EC","crv":"P-256","use":"sig","alg":"ES256","kid":"` + kid + `","x":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","y":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`),
		NotAfter:      notAfter,
	}
}
