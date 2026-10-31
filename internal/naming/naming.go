// Package naming holds the Go identifier rules shared by every feature:
// snake_case to CamelCase conversion (compatible with protoc-gen-go), keyword
// escaping and collision handling.
package naming

import (
	"go/token"
	"unicode"
	"unicode/utf8"
)

// UpperCamel converts a snake_case proto identifier to UpperCamelCase the
// way protoc-gen-go does: each run between underscores is capitalised and
// the underscores dropped, except that an underscore before a digit is kept
// so that "foo_1bar" stays unambiguous ("Foo_1Bar").
func UpperCamel(s string) string {
	b := make([]byte, 0, len(s))
	upper := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_':
			if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' && len(b) > 0 {
				b = append(b, '_')
			}
			upper = true
		case upper && c >= 'a' && c <= 'z':
			b = append(b, c-'a'+'A')
			upper = false
		case c >= '0' && c <= '9':
			b = append(b, c)
			upper = true
		default:
			b = append(b, c)
			upper = false
		}
	}
	return string(b)
}

// LowerFirst lowercases the first rune of s.
func LowerFirst(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}

// IsUpperCamel reports whether s is a non-empty ASCII identifier starting
// with an uppercase letter and containing only letters and digits.
func IsUpperCamel(s string) bool {
	if s == "" || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// Escape returns name with a trailing underscore when it is a Go keyword or
// one of the reserved identifiers, and whether it was changed.
func Escape(name string, reserved map[string]bool) (string, bool) {
	if token.IsKeyword(name) || reserved[name] {
		return name + "_", true
	}
	return name, false
}
