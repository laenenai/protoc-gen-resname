package resname

import (
	"strings"
	"testing"
)

func TestParsePattern(t *testing.T) {
	cases := []struct {
		in        string
		vars      []string
		singleton bool
		parent    string
	}{
		{"publishers/{publisher}", []string{"publisher"}, false, ""},
		{"publishers/{publisher}/books/{book}", []string{"publisher", "book"}, false, "publishers/{publisher}"},
		{"users/{user}/config", []string{"user"}, true, "users/{user}"},
		{"projects/{project}/keyRings/{key_ring}", []string{"project", "key_ring"}, false, "projects/{project}"},
		{"v1beta1Things/{thing}", []string{"thing"}, false, ""},
	}
	for _, c := range cases {
		p, err := parsePattern(c.in)
		if err != nil {
			t.Errorf("parsePattern(%q): %v", c.in, err)
			continue
		}
		if p.Raw != c.in || p.Singleton != c.singleton || p.ParentRaw != c.parent || strings.Join(p.Variables, ",") != strings.Join(c.vars, ",") {
			t.Errorf("parsePattern(%q) = %+v", c.in, p)
		}
	}
}

func TestParsePatternErrors(t *testing.T) {
	cases := map[string]string{
		"":                                        `pattern "" is empty`,
		"{publisher}/books/{book}":                `pattern "{publisher}/books/{book}": must start with a literal collection id`,
		"publishers/{publisher}/{book}":           `pattern "publishers/{publisher}/{book}": segment 3 ({book}) breaks the literal/{variable} alternation`,
		"publishers/books/{book}":                 `pattern "publishers/books/{book}": segment 2 (books) breaks the literal/{variable} alternation`,
		"publishers":                              `pattern "publishers": has no variable`,
		"Publishers/{publisher}":                  `pattern "Publishers/{publisher}": segment 1 (Publishers) is not a collection id ([a-z][a-zA-Z0-9]*)`,
		"publishers/{Publisher}":                  `pattern "publishers/{Publisher}": segment 2 ({Publisher}) is not a variable ({snake_name})`,
		"publishers/{publisher}/books/{publisher}": `pattern "publishers/{publisher}/books/{publisher}": variable {publisher} appears twice`,
		"publishers//{publisher}":                 `pattern "publishers//{publisher}": segment 2 () breaks the literal/{variable} alternation`,
		"/publishers/{publisher}":                 `pattern "/publishers/{publisher}": must start with a literal collection id`,
		"publishers/{publisher}/":                 `pattern "publishers/{publisher}/": segment 3 () is not a collection id ([a-z][a-zA-Z0-9]*)`,
		"publishers/{_publisher}":                 `pattern "publishers/{_publisher}": segment 2 ({_publisher}) is not a variable ({snake_name})`,
		"publishers/{publisher_}":                 `pattern "publishers/{publisher_}": segment 2 ({publisher_}) is not a variable ({snake_name})`,
		"publishers/{a__b}":                       `pattern "publishers/{a__b}": segment 2 ({a__b}) is not a variable ({snake_name})`,
		"publishers/{}":                           `pattern "publishers/{}": segment 2 ({}) is not a variable ({snake_name})`,
	}
	for in, want := range cases {
		_, err := parsePattern(in)
		if err == nil || err.Error() != want {
			t.Errorf("parsePattern(%q) error = %v\n want %s", in, err, want)
		}
	}
}

func TestParseType(t *testing.T) {
	for in, want := range map[string]string{
		"library.example.com/Book":                        "Book",
		"cloudresourcemanager.googleapis.com/Project":     "Project",
		"library.example.com/KeyRing":                     "KeyRing",
		"library.example.com/book":                        "",
		"Book":                                            "",
		"/Book":                                           "",
		"library.example.com/":                            "",
		"library.example.com/Book/Page":                   "",
		"library.example.com/Book_Page":                   "",
		"":                                                "",
		"library example.com/Book":                        "",
	} {
		got, err := parseType(in)
		if want == "" {
			if err == nil || err.Error() != `type "`+in+`" must be {service}/{Kind} with an UpperCamel Kind` {
				t.Errorf("parseType(%q) error = %v", in, err)
			}
			continue
		}
		if err != nil || got != want {
			t.Errorf("parseType(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}
