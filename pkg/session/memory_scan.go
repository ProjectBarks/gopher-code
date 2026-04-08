package session

import (
	"os"
	"path/filepath"
	"strings"
)

// Source: utils/memdir/memoryScanner.ts

// ScanMemoryDir scans a memory directory and returns all .md file paths.
func ScanMemoryDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".md") && name != "MEMORY.md" {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return files, nil
}

// CountMemoryFiles returns the number of memory files in a directory.
func CountMemoryFiles(dir string) int {
	files, err := ScanMemoryDir(dir)
	if err != nil {
		return 0
	}
	return len(files)
}
