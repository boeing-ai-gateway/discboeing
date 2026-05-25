//go:build windows

package wsl

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

func (m *Manager) runCommand(ctx context.Context, name string, args ...string) (string, error) {
	output, err := runCommandOutput(ctx, name, args...)
	trimmed := strings.TrimSpace(output)
	if err != nil {
		if trimmed == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, trimmed)
	}
	return trimmed, nil
}

func decodeCommandOutput(output []byte) string {
	if len(output) >= 2 {
		switch {
		case output[0] == 0xff && output[1] == 0xfe:
			return decodeUTF16(output[2:], binary.LittleEndian)
		case output[0] == 0xfe && output[1] == 0xff:
			return decodeUTF16(output[2:], binary.BigEndian)
		case looksLikeUTF16(output, binary.LittleEndian):
			return decodeUTF16(output, binary.LittleEndian)
		case looksLikeUTF16(output, binary.BigEndian):
			return decodeUTF16(output, binary.BigEndian)
		}
	}

	return string(output)
}

func looksLikeUTF16(output []byte, order binary.ByteOrder) bool {
	if len(output) < 4 || len(output)%2 != 0 {
		return false
	}

	zeroCount := 0
	pairs := len(output) / 2
	for i := 0; i < len(output); i += 2 {
		var candidate byte
		if order == binary.LittleEndian {
			candidate = output[i+1]
		} else {
			candidate = output[i]
		}
		if candidate == 0 {
			zeroCount++
		}
	}

	return zeroCount >= pairs/2
}

func decodeUTF16(output []byte, order binary.ByteOrder) string {
	if len(output)%2 != 0 {
		output = output[:len(output)-1]
	}
	if len(output) == 0 {
		return ""
	}

	words := make([]uint16, len(output)/2)
	for i := 0; i < len(output); i += 2 {
		words[i/2] = order.Uint16(output[i : i+2])
	}

	return string(utf16.Decode(words))
}
