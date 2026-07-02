//go:build !windows

package hcs

import "github.com/boeing-ai-gateway/discboeing/hcs-go/internal/cli"

type VMAccessGrant struct{}

func GrantVMAccess(cli.Options) (*VMAccessGrant, error) { return &VMAccessGrant{}, nil }
func (g *VMAccessGrant) Close() error                   { return nil }
