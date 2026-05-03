// Package id creates stable public IDs for Meta resources.
package id

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"
)

type Type string

const (
	TypeOrganization               Type = "organization"
	TypeOrganizationMember         Type = "organization_member"
	TypeOrganizationBootstrapToken Type = "organization_bootstrap_token"
	TypeOAuthApplication           Type = "oauth_application"
	TypeUser                       Type = "user"
	TypeUserIdentity               Type = "user_identity"
	TypeUserSession                Type = "user_session"
	TypeUserDevice                 Type = "user_device"
	TypeGroup                      Type = "group"
	TypeGroupMember                Type = "group_member"
	TypeGroupIdentity              Type = "group_identity"
	TypeProject                    Type = "project"
	TypeProjectMember              Type = "project_member"
	TypeAgentSession               Type = "agent_session"
	TypeAgentSessionPublicKey      Type = "agent_session_public_key"
	TypeAgentSessionMember         Type = "agent_session_member"
	TypeSecret                     Type = "secret"
	TypeSecretOwner                Type = "secret_owner"
	TypeSecretVersion              Type = "secret_version"
	TypeSecretRecipientWrap        Type = "secret_recipient_wrap"
	TypeSecretBinding              Type = "secret_binding"
	TypeJWTSigningKey              Type = "jwt_signing_key"
	TypeAuditEvent                 Type = "audit_event"
)

var prefixes = map[Type]string{
	TypeOrganization:               "org",
	TypeOrganizationMember:         "ombr",
	TypeOrganizationBootstrapToken: "obt",
	TypeOAuthApplication:           "oapp",
	TypeUser:                       "usr",
	TypeUserIdentity:               "uid",
	TypeUserSession:                "usess",
	TypeUserDevice:                 "dev",
	TypeGroup:                      "grp",
	TypeGroupMember:                "gmbr",
	TypeGroupIdentity:              "gid",
	TypeProject:                    "prj",
	TypeProjectMember:              "pmbr",
	TypeAgentSession:               "ags",
	TypeAgentSessionPublicKey:      "asp",
	TypeAgentSessionMember:         "asm",
	TypeSecret:                     "sec",
	TypeSecretOwner:                "sown",
	TypeSecretVersion:              "sver",
	TypeSecretRecipientWrap:        "swrp",
	TypeSecretBinding:              "sbind",
	TypeJWTSigningKey:              "jwk",
	TypeAuditEvent:                 "aud",
}

const CrockfordBase32 = "0123456789abcdefghjkmnpqrstvwxyz"

// New returns a prefixed ULID for the given Meta resource type.
func New(t Type) (string, error) {
	prefix, ok := prefixes[t]
	if !ok {
		return "", fmt.Errorf("unknown id type %q", t)
	}
	ulid, err := newULID(time.Now())
	if err != nil {
		return "", err
	}
	return prefix + "_" + ulid, nil
}

// MustNew returns a prefixed ULID and panics if generation fails.
func MustNew(t Type) string {
	id, err := New(t)
	if err != nil {
		panic(err)
	}
	return id
}

// Prefix returns the registered prefix for a Meta resource type.
func Prefix(t Type) (string, bool) {
	prefix, ok := prefixes[t]
	return prefix, ok
}

func newULID(now time.Time) (string, error) {
	var data [16]byte
	ms := uint64(now.UTC().UnixMilli())
	data[0] = byte(ms >> 40)
	data[1] = byte(ms >> 32)
	data[2] = byte(ms >> 24)
	data[3] = byte(ms >> 16)
	data[4] = byte(ms >> 8)
	data[5] = byte(ms)
	if _, err := rand.Read(data[6:]); err != nil {
		return "", fmt.Errorf("generate ulid entropy: %w", err)
	}
	return encodeULID(data[:]), nil
}

func encodeULID(data []byte) string {
	value := new(big.Int).SetBytes(data)
	base := big.NewInt(32)
	mod := new(big.Int)
	out := make([]byte, 26)
	for i := len(out) - 1; i >= 0; i-- {
		value.DivMod(value, base, mod)
		out[i] = CrockfordBase32[mod.Int64()]
	}
	return string(out)
}

// RandomCrockford returns size characters from the lowercase Crockford base32
// alphabet using crypto/rand.
func RandomCrockford(size int) (string, error) {
	if size < 0 {
		return "", fmt.Errorf("invalid random string size %d", size)
	}
	out := make([]byte, size)
	var buf [8]byte
	for i := range out {
		if _, err := rand.Read(buf[:]); err != nil {
			return "", fmt.Errorf("generate random crockford: %w", err)
		}
		out[i] = CrockfordBase32[binary.BigEndian.Uint64(buf[:])%uint64(len(CrockfordBase32))]
	}
	return string(out), nil
}
