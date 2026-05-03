package meta

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/obot-platform/discobot/meta/internal/auth"
	"github.com/obot-platform/discobot/meta/internal/model"
	"github.com/obot-platform/discobot/meta/internal/store"
)

type bootstrapResult struct {
	Organization *model.Organization
	Token        string
	CreatedToken bool
	SetupMode    bool
}

func ensurePublicOrganizationBootstrap(ctx context.Context, st *store.Store) (*bootstrapResult, error) {
	org, err := st.GetOrganizationByDomain(ctx, model.PublicOrganizationDomain)
	if errors.Is(err, store.ErrNotFound) {
		org = &model.Organization{
			Name:   "Public",
			Domain: model.PublicOrganizationDomain,
		}
		if err := st.CreateOrganization(ctx, org); err != nil {
			return nil, fmt.Errorf("create public organization: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("load public organization: %w", err)
	}

	initialized, err := st.HasOrganizationOwnerOrAdmin(ctx, org.ID)
	if err != nil {
		return nil, fmt.Errorf("check public organization initialization: %w", err)
	}
	result := &bootstrapResult{Organization: org, SetupMode: !initialized}
	if initialized {
		if err := st.RevokeActiveOrganizationBootstrapTokens(ctx, org.ID, time.Now()); err != nil {
			return nil, fmt.Errorf("revoke initialized public organization bootstrap tokens: %w", err)
		}
		return result, nil
	}

	now := time.Now()
	if err := st.RevokeActiveOrganizationBootstrapTokens(ctx, org.ID, now); err != nil {
		return nil, fmt.Errorf("revoke old public organization bootstrap tokens: %w", err)
	}

	raw, hash, err := auth.NewBootstrapToken()
	if err != nil {
		return nil, err
	}
	description := "Initial public organization bootstrap token"
	token := &model.OrganizationBootstrapToken{
		OrganizationID: org.ID,
		TokenHash:      hash,
		Description:    &description,
	}
	if err := st.CreateOrganizationBootstrapToken(ctx, token); err != nil {
		return nil, fmt.Errorf("create public organization bootstrap token: %w", err)
	}
	result.Token = raw
	result.CreatedToken = true
	return result, nil
}

func logBootstrapResult(result *bootstrapResult) {
	if result == nil || result.Organization == nil {
		return
	}
	if !result.SetupMode {
		log.Printf("meta public organization %q (%s) is initialized", result.Organization.Domain, result.Organization.ID)
		return
	}
	if result.CreatedToken {
		log.Printf("meta setup mode: regenerated bootstrap token for organization %q (%s): %s", result.Organization.Domain, result.Organization.ID, result.Token)
		log.Printf("use this token as: Authorization: Bearer %s", result.Token)
		return
	}
	log.Printf("meta setup mode is active for organization %q (%s), but no bootstrap token was generated", result.Organization.Domain, result.Organization.ID)
}
