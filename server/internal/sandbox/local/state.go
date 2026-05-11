package local

import (
	"context"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

func (p *Provider) PrepareState(context.Context, string, sandbox.CreateOptions) ([]byte, error) {
	return nil, nil
}
