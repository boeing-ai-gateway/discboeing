//go:build !windows

package hcn

import (
	"fmt"

	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/cli"
	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/hcs"
)

type NATConnection struct {
	Properties   EndpointProperties
	NetworkJSON  string
	EndpointJSON string
	AdapterJSON  string
}

func CreateNAT(cli.Options) (*NATConnection, error) {
	return nil, fmt.Errorf("Windows HCN APIs are only available on Windows")
}

func (n *NATConnection) Attach(hcs.ComputeSystem) error {
	return fmt.Errorf("Windows HCN APIs are only available on Windows")
}
func (n *NATConnection) Close() error { return nil }
