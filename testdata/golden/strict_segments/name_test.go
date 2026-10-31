package strict_segments

import (
	"strings"
	"testing"
)

func TestStrictSegments(t *testing.T) {
	if _, err := ParseBookName("publishers/acme-1/books/dune-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseBookName("publishers/-/books/dune"); err != nil {
		t.Fatalf("wildcard rejected by Parse: %v", err)
	}
	long := strings.Repeat("a", 64)
	for in, want := range map[string]string{
		"publishers/Acme/books/dune":    `invalid resource name: book name "publishers/Acme/books/dune": segment {publisher} must match [a-z0-9-]{1,63}`,
		"publishers/acme/books/du_ne":   `invalid resource name: book name "publishers/acme/books/du_ne": segment {book} must match [a-z0-9-]{1,63}`,
		"publishers/acme/books/" + long: `invalid resource name: book name "publishers/acme/books/` + long + `": segment {book} must match [a-z0-9-]{1,63}`,
		"publishers/ü/books/dune":       `invalid resource name: book name "publishers/ü/books/dune": segment {publisher} must match [a-z0-9-]{1,63}`,
		"publishers//books/dune":        `invalid resource name: book name "publishers//books/dune": segment {publisher} is empty`,
	} {
		if _, err := ParseBookName(in); err == nil || err.Error() != want {
			t.Errorf("ParseBookName(%q) = %v\n   want %s", in, err, want)
		}
	}
	if err := (BookName{Publisher: "acme", Book: strings.Repeat("b", 63)}).Validate(); err != nil {
		t.Errorf("63-char segment rejected: %v", err)
	}
}
