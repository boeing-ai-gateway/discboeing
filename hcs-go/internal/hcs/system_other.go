//go:build !windows

package hcs

import (
	"fmt"

	"github.com/google/uuid"
)

type System struct{}

func CreateComputeSystem(uuid.UUID, string) (*System, error) {
	return nil, fmt.Errorf("Windows HCS APIs are only available on Windows")
}

func (s *System) ID() uuid.UUID { return uuid.Nil }
func (s *System) Start() error  { return fmt.Errorf("Windows HCS APIs are only available on Windows") }
func (s *System) Modify(string) error {
	return fmt.Errorf("Windows HCS APIs are only available on Windows")
}
func (s *System) GetProperties(string) (string, error) {
	return "", fmt.Errorf("Windows HCS APIs are only available on Windows")
}
func (s *System) Terminate() error { return nil }
func (s *System) Close() error     { return nil }
