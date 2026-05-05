// Package protocol contains ACP protocol types generated from the checked-in
// ACP JSON Schema snapshot.
package protocol

//go:generate go run ../internal/cmd/acpgen -schema ../schema/schema.json -out types_gen.go
