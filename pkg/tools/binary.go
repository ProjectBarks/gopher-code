package tools

import (
	"path/filepath"
	"strings"
)

// Source: constants/files.ts

// BinaryExtensions is the set of file extensions that indicate binary content.
// These files can't be meaningfully compared as text and are often large.
var BinaryExtensions = map[string]bool{
	// Images
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
	".ico": true, ".webp": true, ".tiff": true, ".tif": true,
	// Videos
	".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true,
	".wmv": true, ".flv": true, ".m4v": true, ".mpeg": true, ".mpg": true,
	// Audio
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".aac": true,
	".m4a": true, ".wma": true, ".aiff": true, ".opus": true,
	// Archives
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".7z": true,
	".rar": true, ".xz": true, ".z": true, ".tgz": true, ".iso": true,
	// Executables/binaries
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true,
	".o": true, ".a": true, ".obj": true, ".lib": true, ".app": true,
	".msi": true, ".deb": true, ".rpm": true,
	// Documents
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".odt": true, ".ods": true, ".odp": true,
	// Fonts
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
	// Bytecode / VM artifacts
	".pyc": true, ".pyo": true, ".class": true, ".jar": true, ".war": true,
	".ear": true, ".node": true, ".wasm": true, ".rlib": true,
	// Database files
	".sqlite": true, ".sqlite3": true, ".db": true, ".mdb": true, ".idx": true,
	// Design / 3D
	".psd": true, ".ai": true, ".eps": true, ".sketch": true, ".fig": true,
	".xd": true, ".blend": true, ".3ds": true, ".max": true,
	// Flash
	".swf": true, ".fla": true,
	// Lock/profiling data
	".lockb": true, ".dat": true, ".data": true,
}

// HasBinaryExtension returns true if the file path has a known binary extension.
// Source: files.ts:117-120
func HasBinaryExtension(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return BinaryExtensions[ext]
}

// BinaryCheckSize is the number of bytes to read for binary content detection.
const BinaryCheckSize = 8192

// IsBinaryContent checks if data contains binary content by looking for null
// bytes or a high proportion of non-printable characters.
// Source: files.ts:131-156
func IsBinaryContent(data []byte) bool {
	checkSize := len(data)
	if checkSize > BinaryCheckSize {
		checkSize = BinaryCheckSize
	}
	if checkSize == 0 {
		return false
	}

	nonPrintable := 0
	for i := 0; i < checkSize; i++ {
		b := data[i]
		// Null byte is a strong indicator of binary.
		if b == 0 {
			return true
		}
		// Count non-printable, non-whitespace bytes.
		// Printable ASCII is 32-126, plus tab (9), newline (10), CR (13).
		if b < 32 && b != 9 && b != 10 && b != 13 {
			nonPrintable++
		}
	}

	// If more than 10% non-printable, likely binary.
	return float64(nonPrintable)/float64(checkSize) > 0.1
}
