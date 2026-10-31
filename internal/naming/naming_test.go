package naming

import "testing"

func TestUpperCamel(t *testing.T) {
	for in, want := range map[string]string{
		"book":        "Book",
		"key_ring":    "KeyRing",
		"publisher":   "Publisher",
		"crypto_key1": "CryptoKey1",
		"foo_1bar":    "Foo_1Bar",
		"__x":         "X",
		"a_b_c":       "ABC",
		"":            "",
	} {
		if got := UpperCamel(in); got != want {
			t.Errorf("UpperCamel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLowerFirst(t *testing.T) {
	for in, want := range map[string]string{
		"Book": "book", "UserConfig": "userConfig", "": "", "KeyRing": "keyRing",
	} {
		if got := LowerFirst(in); got != want {
			t.Errorf("LowerFirst(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsUpperCamel(t *testing.T) {
	for in, want := range map[string]bool{
		"Book": true, "UserConfig": true, "B2": true,
		"book": false, "": false, "Book_Name": false, "User-Config": false, "Ümlaut": false,
	} {
		if got := IsUpperCamel(in); got != want {
			t.Errorf("IsUpperCamel(%q) = %v, want %v", in, got, want)
		}
	}
}

// Escape appends an underscore when the candidate is a Go keyword or one of
// the names the caller reserves (e.g. the methods on a generated type).
func TestEscape(t *testing.T) {
	reserved := map[string]bool{"String": true, "Type": true}
	for in, want := range map[string]string{
		"Book": "Book", "String": "String_", "Type": "Type_", "type": "type_", "range": "range_", "Range": "Range",
	} {
		got, escaped := Escape(in, reserved)
		if got != want || escaped != (got != in) {
			t.Errorf("Escape(%q) = %q, %v; want %q", in, got, escaped, want)
		}
	}
}
