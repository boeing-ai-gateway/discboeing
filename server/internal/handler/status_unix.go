//go:build unix

package handler

import (
	"github.com/obot-platform/discobot/server/client"

	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// getDiskUsage returns filesystem usage statistics for a given path
func getDiskUsage(path string) *client.DiskUsageInfo {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	availableBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - (stat.Bfree * uint64(stat.Bsize))

	var usedPercent float64
	if totalBytes > 0 {
		usedPercent = float64(usedBytes) / float64(totalBytes) * 100
	}

	return &client.DiskUsageInfo{
		TotalBytes:     totalBytes,
		UsedBytes:      usedBytes,
		AvailableBytes: availableBytes,
		UsedPercent:    usedPercent,
	}
}

// getDataDiskFiles scans for project data disk images and returns their size info.
// Data disks are sparse files, so actual disk usage may be much less than apparent size.
func getDataDiskFiles(dataDir string) []client.DataDiskFileInfo {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil
	}

	var disks []client.DataDiskFileInfo
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "project-") || !strings.HasSuffix(name, "-data.img") {
			continue
		}

		path := filepath.Join(dataDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		apparentBytes := uint64(info.Size())

		// Get actual disk usage via stat blocks (sparse-aware)
		var stat syscall.Stat_t
		var actualBytes uint64
		if err := syscall.Stat(path, &stat); err == nil {
			actualBytes = uint64(stat.Blocks) * 512
		}

		disks = append(disks, client.DataDiskFileInfo{
			Path:          path,
			ApparentBytes: apparentBytes,
			ActualBytes:   actualBytes,
		})
	}

	return disks
}
