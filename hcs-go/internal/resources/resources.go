package resources

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const (
	mib             = 1024 * 1024
	minimumMemoryMB = 128
)

func DefaultProcessorCount() int {
	if n := runtime.NumCPU(); n > 0 {
		return n
	}
	return 1
}

func DefaultMemoryMB() int {
	halfHostMemory := totalPhysicalMemoryBytes() / 2
	if halfHostMemory <= 0 {
		return 2048
	}

	memoryMB := halfHostMemory / mib
	if memoryMB > math.MaxInt32 {
		memoryMB = math.MaxInt32
	}
	if memoryMB < minimumMemoryMB {
		memoryMB = minimumMemoryMB
	}
	return AlignMemoryMB(int(memoryMB))
}

func MemoryGBToMB(memoryGB float64) (int, error) {
	if memoryGB <= 0 {
		return 0, fmt.Errorf("memory size must be greater than zero")
	}
	memoryMB := math.Ceil(memoryGB * 1024)
	if memoryMB > math.MaxInt32 {
		return 0, fmt.Errorf("memory size is too large")
	}
	return AlignMemoryMB(int(memoryMB)), nil
}

func AlignMemoryMB(memoryMB int) int {
	return memoryMB &^ 1
}

func totalPhysicalMemoryBytes() int64 {
	if value, ok := readProcMeminfo(); ok {
		return value
	}
	return 0
}

func readProcMeminfo() (int64, bool) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "MemTotal:"))
		if len(fields) == 0 {
			return 0, false
		}
		kib, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return 0, false
		}
		return kib * 1024, true
	}
	return 0, false
}
