package vz

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"

	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
)

const (
	vzKernelArtifact   = "vmlinuz"
	vzBaseDiskArtifact = "discobot-rootfs.squashfs"
)

func newImageDownloader(imageRef string, dataDir string) *vm.ImageDownloader {
	return vm.NewImageDownloader(vm.ImageDownloadConfig{
		ImageRef:            imageRef,
		DataDir:             dataDir,
		ProviderName:        "VZ",
		ArtifactDescription: "VZ kernel and base disk images",
		Artifacts: []vm.ImageArtifactSpec{
			{Name: vzKernelArtifact},
			{Name: vzBaseDiskArtifact},
		},
		PostProcess: func(paths map[string]string) error {
			return decompressKernel(paths[vzKernelArtifact])
		},
	})
}

// compressionFormat describes a known kernel compression format.
type compressionFormat struct {
	name       string
	magic      []byte
	decompress func([]byte) ([]byte, error)
}

// knownFormats lists compression formats to scan for inside vmlinuz.
var knownFormats = []compressionFormat{
	{"gzip", []byte{0x1f, 0x8b, 0x08}, func(data []byte) ([]byte, error) {
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	}},
	{"zstd", []byte{0x28, 0xb5, 0x2f, 0xfd}, func(data []byte) ([]byte, error) {
		// The kernel's zstd compressor uses 128MB windows.
		dec, err := zstd.NewReader(nil, zstd.WithDecoderMaxWindow(1<<31), zstd.WithDecoderConcurrency(1))
		if err != nil {
			return nil, err
		}
		defer dec.Close()
		// DecodeAll decodes all concatenated frames. When the compressed kernel
		// is embedded in a vmlinuz payload, trailing bytes after the frame cause
		// a "magic number mismatch" error on the non-existent second frame.
		// The first frame's data is still returned, so use it if valid.
		out, decErr := dec.DecodeAll(data, nil)
		if len(out) > 0 {
			return out, nil
		}
		return nil, decErr
	}},
	{"xz", []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}, func(data []byte) ([]byte, error) {
		r, err := xz.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return io.ReadAll(r)
	}},
	{"lzma", []byte{0x5d, 0x00, 0x00}, func(data []byte) ([]byte, error) {
		r, err := lzma.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return io.ReadAll(r)
	}},
}

// isKernelImage checks if data is a valid uncompressed Linux kernel.
// Supports x86_64 ELF and ARM64 Image formats.
func isKernelImage(data []byte) bool {
	// x86_64 ELF: starts with 0x7f ELF
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		return true
	}
	// ARM64 Image: has "ARMd" magic at offset 0x38
	if len(data) > 0x3c && bytes.Equal(data[0x38:0x3c], []byte("ARMd")) {
		return true
	}
	return false
}

// decompressKernel extracts an uncompressed kernel from a vmlinuz file.
// Apple Virtualization framework requires an uncompressed kernel image
// (ELF for x86_64 or Image for ARM64).
//
// vmlinuz files may be:
// 1. Already uncompressed (ELF or ARM64 Image)
// 2. Directly compressed (gzip/zstd/xz/lzma at offset 0)
// 3. An x86 PE/EFI boot stub with compressed payload embedded at an offset
//
// For case 3, we use the Linux boot protocol header to locate the payload,
// then fall back to scanning for compression magic bytes throughout the file
// (like the kernel's scripts/extract-vmlinux).
func decompressKernel(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Case 1: already uncompressed kernel
	if isKernelImage(data) {
		log.Printf("Kernel is already uncompressed")
		return nil
	}

	// Case 2: directly compressed (e.g., ARM64 gzip vmlinuz)
	for _, cf := range knownFormats {
		if len(data) < len(cf.magic) || !bytes.Equal(data[:len(cf.magic)], cf.magic) {
			continue
		}
		log.Printf("vmlinuz is directly %s compressed, decompressing", cf.name)
		decompressed, err := cf.decompress(data)
		if err != nil {
			log.Printf("Direct %s decompression failed: %v", cf.name, err)
			break // fall through to other methods
		}
		if isKernelImage(decompressed) {
			log.Printf("Successfully decompressed %s kernel (%d bytes)", cf.name, len(decompressed))
			return os.WriteFile(path, decompressed, 0644)
		}
		log.Printf("Direct %s decompression produced non-kernel data, continuing", cf.name)
		break
	}

	// Case 3: x86 PE/EFI stub — use the Linux boot protocol header to find the payload.
	if len(data) > 0x250 && bytes.Equal(data[0x202:0x206], []byte("HdrS")) {
		setupSects := int(data[0x1f1])
		if setupSects == 0 {
			setupSects = 4 // default per boot protocol
		}
		protectedModeStart := (setupSects + 1) * 512
		payloadOffset := int(binary.LittleEndian.Uint32(data[0x248:0x24c]))
		payloadLength := int(binary.LittleEndian.Uint32(data[0x24c:0x250]))

		absOffset := protectedModeStart + payloadOffset
		absEnd := absOffset + payloadLength

		log.Printf("Linux boot protocol: setup_sects=%d, protected_mode_start=%d, payload_offset=%d, payload_length=%d, abs_offset=%d",
			setupSects, protectedModeStart, payloadOffset, payloadLength, absOffset)

		if absOffset > 0 && absEnd <= len(data) && payloadLength > 0 {
			payload := data[absOffset:absEnd]
			result, err := tryDecompress(payload)
			if err == nil {
				log.Printf("Successfully extracted kernel (%d bytes) via boot protocol header", len(result))
				return os.WriteFile(path, result, 0644)
			}
			log.Printf("Boot protocol payload decompression failed: %v, falling back to magic scan", err)
		} else {
			log.Printf("Boot protocol header has invalid offsets, falling back to magic scan")
		}
	}

	// Fallback: scan for compression magic bytes throughout the file
	// (like the kernel's scripts/extract-vmlinux).
	for _, cf := range knownFormats {
		searchFrom := 0
		for {
			idx := bytes.Index(data[searchFrom:], cf.magic)
			if idx < 0 {
				break
			}
			offset := searchFrom + idx
			searchFrom = offset + 1

			log.Printf("Found %s signature at offset %d, attempting decompression", cf.name, offset)

			decompressed, err := cf.decompress(data[offset:])
			if err != nil {
				log.Printf("Failed to decompress %s at offset %d: %v", cf.name, offset, err)
				continue
			}

			if !isKernelImage(decompressed) {
				log.Printf("Decompressed %s at offset %d did not produce valid kernel, skipping", cf.name, offset)
				continue
			}

			log.Printf("Successfully extracted kernel (%d bytes) from %s at offset %d", len(decompressed), cf.name, offset)
			return os.WriteFile(path, decompressed, 0644)
		}
	}

	return fmt.Errorf("could not extract kernel from vmlinuz (file starts with %x)", data[:min(4, len(data))])
}

// tryDecompress attempts to decompress data using each known format.
func tryDecompress(data []byte) ([]byte, error) {
	for _, cf := range knownFormats {
		if len(data) < len(cf.magic) || !bytes.Equal(data[:len(cf.magic)], cf.magic) {
			continue
		}

		log.Printf("Payload matches %s format", cf.name)
		decompressed, err := cf.decompress(data)
		if err != nil {
			return nil, fmt.Errorf("%s decompress: %w", cf.name, err)
		}

		if isKernelImage(decompressed) {
			return decompressed, nil
		}
		return nil, fmt.Errorf("%s decompressed data is not a valid kernel (starts with %x)", cf.name, decompressed[:min(4, len(decompressed))])
	}

	// No magic match — try all formats anyway (payload may have a small header before compression)
	for _, cf := range knownFormats {
		idx := bytes.Index(data, cf.magic)
		if idx < 0 || idx > 1024 {
			continue
		}
		log.Printf("Found %s signature at payload offset %d", cf.name, idx)
		decompressed, err := cf.decompress(data[idx:])
		if err != nil {
			continue
		}
		if isKernelImage(decompressed) {
			return decompressed, nil
		}
	}

	return nil, fmt.Errorf("no known compression format matched payload (starts with %x)", data[:min(4, len(data))])
}
