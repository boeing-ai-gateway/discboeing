package vm

import (
	"context"

	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
)

func (p *Provider) PrepareState(context.Context, string, sandbox.CreateOptions) ([]byte, error) {
	return nil, nil
}
