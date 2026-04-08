package session

import (
	"os"
	"time"
)

// Source: utils/memdir/memoryAge.ts

// MemoryAge returns the age of a memory file based on its modification time.
func MemoryAge(path string) time.Duration {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return time.Since(info.ModTime())
}

// IsMemoryStale returns true if the memory file is older than the threshold.
func IsMemoryStale(path string, threshold time.Duration) bool {
	age := MemoryAge(path)
	return age > 0 && age > threshold
}

// MemoryAgeDays returns the age in whole days.
func MemoryAgeDays(path string) int {
	age := MemoryAge(path)
	if age <= 0 {
		return 0
	}
	return int(age.Hours() / 24)
}
