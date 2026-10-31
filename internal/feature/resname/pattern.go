package resname

import (
	"fmt"
	"strings"

	"github.com/laenenai/protoc-gen-resname/internal/naming"
)

// Segment is one "/"-separated element of a resource-name pattern: either a
// literal collection id or a {variable}.
type Segment struct {
	Literal  string // set when the segment is a collection id
	Variable string // set when the segment is a {variable}; the name without braces
}

// IsVariable reports whether the segment is a {variable}.
func (s Segment) IsVariable() bool { return s.Variable != "" }

// Pattern is a parsed, validated resource-name pattern (spec §4.3).
type Pattern struct {
	Raw       string
	Segments  []Segment
	Variables []string // variable names in order
	Singleton bool     // pattern ends in a literal (e.g. users/{user}/config)
	ParentRaw string   // the pattern with the last collection/{variable} (or trailing literal) removed; "" for top-level
}

// parsePattern validates a pattern against the grammar of spec §4.3:
//
//	pattern  := segment ("/" segment)*
//	segment  := literal | variable
//	literal  := [a-z][a-zA-Z0-9]*
//	variable := "{" snake_name "}"
//
// Segments must alternate literal/{variable}; a trailing literal makes the
// pattern a singleton. Variable names are unique within a pattern.
func parsePattern(raw string) (*Pattern, error) {
	if raw == "" {
		return nil, fmt.Errorf("pattern %q is empty", raw)
	}
	parts := strings.Split(raw, "/")
	p := &Pattern{Raw: raw}
	seen := map[string]bool{}
	for i, part := range parts {
		n := i + 1
		isVar := strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && len(part) >= 2
		if i == 0 && (isVar || part == "") {
			return nil, fmt.Errorf("pattern %q: must start with a literal collection id", raw)
		}
		if i > 0 && isVar == p.Segments[i-1].IsVariable() {
			return nil, fmt.Errorf("pattern %q: segment %d (%s) breaks the literal/{variable} alternation", raw, n, part)
		}
		if isVar {
			name := part[1 : len(part)-1]
			if !isSnake(name) {
				return nil, fmt.Errorf("pattern %q: segment %d (%s) is not a variable ({snake_name})", raw, n, part)
			}
			if seen[name] {
				return nil, fmt.Errorf("pattern %q: variable {%s} appears twice", raw, name)
			}
			seen[name] = true
			p.Segments = append(p.Segments, Segment{Variable: name})
			p.Variables = append(p.Variables, name)
			continue
		}
		if !isCollectionID(part) {
			return nil, fmt.Errorf("pattern %q: segment %d (%s) is not a collection id ([a-z][a-zA-Z0-9]*)", raw, n, part)
		}
		p.Segments = append(p.Segments, Segment{Literal: part})
	}
	if len(p.Variables) == 0 {
		return nil, fmt.Errorf("pattern %q: has no variable", raw)
	}
	last := p.Segments[len(p.Segments)-1]
	p.Singleton = !last.IsVariable()
	switch {
	case p.Singleton:
		p.ParentRaw = strings.Join(parts[:len(parts)-1], "/")
	case len(parts) > 2:
		p.ParentRaw = strings.Join(parts[:len(parts)-2], "/")
	}
	return p, nil
}

// parseType validates a resource type of the form {service}/{Kind} and
// returns the Kind.
func parseType(t string) (string, error) {
	i := strings.LastIndexByte(t, '/')
	if i <= 0 || strings.Count(t, "/") != 1 {
		return "", typeError(t)
	}
	service, kind := t[:i], t[i+1:]
	if strings.ContainsAny(service, " \t\n") || !naming.IsUpperCamel(kind) {
		return "", typeError(t)
	}
	return kind, nil
}

func typeError(t string) error {
	return fmt.Errorf("type %q must be {service}/{Kind} with an UpperCamel Kind", t)
}

// isCollectionID matches [a-z][a-zA-Z0-9]*.
func isCollectionID(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// isSnake matches [a-z][a-z0-9]*(_[a-z0-9]+)*.
func isSnake(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	prevUnderscore := false
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_':
			if prevUnderscore {
				return false
			}
			prevUnderscore = true
		case c >= 'a' && c <= 'z' || c >= '0' && c <= '9':
			prevUnderscore = false
		default:
			return false
		}
	}
	return !prevUnderscore
}
