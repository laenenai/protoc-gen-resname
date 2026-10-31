package nowildcards

import "testing"

func TestWildcardsRejectedByParse(t *testing.T) {
	want := `invalid resource name: book name "publishers/-/books/dune": segment {publisher} must not be a wildcard`
	if _, err := ParseBookName("publishers/-/books/dune"); err == nil || err.Error() != want {
		t.Fatalf("ParseBookName = %v\n   want %s", err, want)
	}
	// ValidateAllowWildcard is still available on a hand-built value.
	if err := (BookName{Publisher: "-", Book: "dune"}).ValidateAllowWildcard(); err != nil {
		t.Fatal(err)
	}
}
