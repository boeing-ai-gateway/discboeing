package hcs

import (
	"fmt"

	"github.com/google/uuid"
)

type ComputeSystem interface {
	ID() uuid.UUID
	Start() error
	Modify(configuration string) error
	GetProperties(query string) (string, error)
	Terminate() error
	Close() error
}

func NewHcsError(operation string, hr uintptr, detail string) error {
	if detail != "" {
		return fmt.Errorf("%s failed with HRESULT 0x%08x: %s", operation, uint32(hr), detail)
	}
	return fmt.Errorf("%s failed with HRESULT 0x%08x", operation, uint32(hr))
}

func Failed(hr uintptr) bool {
	return int32(uint32(hr)) < 0
}
