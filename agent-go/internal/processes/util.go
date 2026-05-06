package processes

import (
	"bytes"
	"encoding/json"
	"os"
	"sync"
)

type ringBuffer struct {
	mu   sync.Mutex
	data []byte
	size int
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{size: size}
}

func (b *ringBuffer) write(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(data) >= b.size {
		b.data = append(b.data[:0], data[len(data)-b.size:]...)
		return
	}
	b.data = append(b.data, data...)
	if over := len(b.data) - b.size; over > 0 {
		copy(b.data, b.data[over:])
		b.data = b.data[:len(b.data)-over]
	}
}

func (b *ringBuffer) snapshot() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	return append([]byte(nil), b.data...)
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func bytesLines(data []byte) [][]byte {
	lines := bytes.Split(data, []byte{'\n'})
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func jsonUnmarshal(data []byte, out any) error {
	return json.Unmarshal(data, out)
}
