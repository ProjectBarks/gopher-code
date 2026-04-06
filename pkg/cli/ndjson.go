package cli

import (
	"encoding/json"
	"strings"
)

// NdjsonSafeStringify marshals value to JSON and escapes U+2028 LINE SEPARATOR
// and U+2029 PARAGRAPH SEPARATOR to their \uXXXX form. These characters are
// valid JSON (ECMA-404) but act as line terminators in some receivers, which
// would split an NDJSON line mid-string and silently corrupt the message.
func NdjsonSafeStringify(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		// Fallback: marshal the error string itself.
		b, _ = json.Marshal(err.Error())
	}
	s := string(b)
	s = strings.ReplaceAll(s, "\u2028", `\u2028`)
	s = strings.ReplaceAll(s, "\u2029", `\u2029`)
	return s
}
