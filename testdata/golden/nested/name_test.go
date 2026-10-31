package nested

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseBookNameRoundTrip(t *testing.T) {
	for _, in := range []string{
		"publishers/acme/books/dune",
		"publishers/a-b_c.d/books/x%20y",
		"publishers/-/books/dune",
		"publishers/acme/books/-",
		"publishers/ümlaut/books/日本",
	} {
		n, err := ParseBookName(in)
		if err != nil {
			t.Fatalf("ParseBookName(%q): %v", in, err)
		}
		if got := n.String(); got != in {
			t.Errorf("String() = %q, want %q", got, in)
		}
		again, err := ParseBookName(n.String())
		if err != nil || again != n {
			t.Errorf("round trip of %q = %+v, %v", in, again, err)
		}
		var u BookName
		if err := u.UnmarshalString(in); err != nil || u != n {
			t.Errorf("UnmarshalString(%q) = %+v, %v", in, u, err)
		}
	}
}

func TestParseBookNameErrors(t *testing.T) {
	cases := map[string]string{
		"":                                 `invalid resource name: book name "": expected 4 segments, got 1`,
		"publishers/acme":                  `invalid resource name: book name "publishers/acme": expected 4 segments, got 2`,
		"publishers/acme/books/dune/extra": `invalid resource name: book name "publishers/acme/books/dune/extra": expected 4 segments, got 5`,
		"shelves/acme/books/dune":          `invalid resource name: book name "shelves/acme/books/dune": expected "publishers" at segment 1, got "shelves"`,
		"publishers/acme/novels/dune":      `invalid resource name: book name "publishers/acme/novels/dune": expected "books" at segment 3, got "novels"`,
		"publishers//books/dune":           `invalid resource name: book name "publishers//books/dune": segment {publisher} is empty`,
		"publishers/acme/books/":           `invalid resource name: book name "publishers/acme/books/": segment {book} is empty`,
		"publishers/acme/books/\xff":       `invalid resource name: book name "publishers/acme/books/\xff": segment {book} is not valid UTF-8`,
		"Publishers/acme/books/dune":       `invalid resource name: book name "Publishers/acme/books/dune": expected "publishers" at segment 1, got "Publishers"`,
		"publishers/acme/books/dune/":      `invalid resource name: book name "publishers/acme/books/dune/": expected 4 segments, got 5`,
		"/publishers/acme/books/dune":      `invalid resource name: book name "/publishers/acme/books/dune": expected 4 segments, got 5`,
	}
	for in, want := range cases {
		_, err := ParseBookName(in)
		if err == nil || err.Error() != want {
			t.Errorf("ParseBookName(%q) error = %v\n   want %s", in, err, want)
		}
		if !errors.Is(err, ErrInvalidResourceName) {
			t.Errorf("ParseBookName(%q) error does not wrap ErrInvalidResourceName", in)
		}
	}
}

func TestBookNameValidate(t *testing.T) {
	ok := BookName{Publisher: "acme", Book: "dune"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	if s, err := ok.MarshalString(); err != nil || s != "publishers/acme/books/dune" {
		t.Fatalf("MarshalString = %q, %v", s, err)
	}
	cases := map[string]struct {
		n    BookName
		want string
	}{
		"empty publisher": {BookName{Book: "dune"}, `invalid resource name: book name "publishers//books/dune": segment {publisher} is empty`},
		"empty book":      {BookName{Publisher: "acme"}, `invalid resource name: book name "publishers/acme/books/": segment {book} is empty`},
		"slash":           {BookName{Publisher: "a/b", Book: "dune"}, `invalid resource name: book name "publishers/a/b/books/dune": segment {publisher} must not contain a slash`},
		"wildcard":        {BookName{Publisher: "-", Book: "dune"}, `invalid resource name: book name "publishers/-/books/dune": segment {publisher} must not be a wildcard`},
		"bad utf8":        {BookName{Publisher: "acme", Book: "\xff"}, `invalid resource name: book name "publishers/acme/books/\xff": segment {book} is not valid UTF-8`},
	}
	for name, c := range cases {
		err := c.n.Validate()
		if err == nil || err.Error() != c.want {
			t.Errorf("%s: Validate() = %v\n   want %s", name, err, c.want)
		}
		if _, err := c.n.MarshalString(); err == nil {
			t.Errorf("%s: MarshalString() succeeded", name)
		}
	}
}

func TestBookNameWildcards(t *testing.T) {
	n := MustParseBookName("publishers/-/books/dune")
	if !n.ContainsWildcard() {
		t.Fatal("ContainsWildcard() = false")
	}
	if err := n.Validate(); err == nil {
		t.Fatal("Validate() accepted a wildcard")
	}
	if err := n.ValidateAllowWildcard(); err != nil {
		t.Fatalf("ValidateAllowWildcard() = %v", err)
	}
	if MustParseBookName("publishers/acme/books/dune").ContainsWildcard() {
		t.Fatal("ContainsWildcard() = true without wildcard")
	}
}

func TestBookNameParentTypeZero(t *testing.T) {
	n := MustParseBookName("publishers/acme/books/dune")
	if p := n.Parent(); p != (PublisherName{Publisher: "acme"}) || p.String() != "publishers/acme" {
		t.Errorf("Parent() = %+v", p)
	}
	if n.Type() != BookType || BookType != "library.example.com/Book" {
		t.Errorf("Type() = %q", n.Type())
	}
	if n.IsZero() || !(BookName{}).IsZero() {
		t.Error("IsZero wrong")
	}
	if BookNamePattern != "publishers/{publisher}/books/{book}" {
		t.Errorf("BookNamePattern = %q", BookNamePattern)
	}
}

func TestMustParsePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustParseBookName did not panic")
		}
	}()
	MustParseBookName("nope")
}

func TestMessageAccessors(t *testing.T) {
	book := &Book{Name: "publishers/acme/books/dune"}
	n, err := book.ResourceName()
	if err != nil || n.Book != "dune" {
		t.Fatalf("ResourceName() = %+v, %v", n, err)
	}
	if book.MustResourceName() != n {
		t.Fatal("MustResourceName differs")
	}
	if _, err := (&Book{Name: "x"}).ResourceName(); !errors.Is(err, ErrInvalidResourceName) {
		t.Fatalf("ResourceName() error = %v", err)
	}

	get := &GetBookRequest{Name: "publishers/acme/books/dune"}
	if n, err := get.BookName(); err != nil || n != MustParseBookName(get.Name) {
		t.Fatalf("BookName() = %+v, %v", n, err)
	}

	batch := &BatchGetBooksRequest{Parent: "publishers/acme", Names: []string{"publishers/acme/books/a", "publishers/acme/books/b"}}
	if p, err := batch.ParentName(); err != nil || p.Publisher != "acme" {
		t.Fatalf("ParentName() = %+v, %v", p, err)
	}
	names, err := batch.BookNames()
	if err != nil || len(names) != 2 || names[1].Book != "b" {
		t.Fatalf("BookNames() = %+v, %v", names, err)
	}
	batch.Names = append(batch.Names, "bad")
	_, err = batch.BookNames()
	if err == nil || !strings.HasPrefix(err.Error(), "names[2]: invalid resource name:") || !errors.Is(err, ErrInvalidResourceName) {
		t.Fatalf("BookNames() error = %v", err)
	}
	if names, err := (&BatchGetBooksRequest{}).BookNames(); err != nil || len(names) != 0 {
		t.Fatalf("BookNames() on empty = %v, %v", names, err)
	}

	mv := &MoveBookRequest{Name: "publishers/acme/books/dune", DestinationName: "publishers/other"}
	if d, err := mv.DestinationPublisherName(); err != nil || d.Publisher != "other" {
		t.Fatalf("DestinationPublisherName() = %+v, %v", d, err)
	}
}

// FuzzParseBookName: no panics, and every accepted input round-trips
// through String() and satisfies ValidateAllowWildcard().
func FuzzParseBookName(f *testing.F) {
	for _, seed := range []string{
		"publishers/acme/books/dune", "publishers/-/books/-", "", "a/b/c/d", "publishers//books/x",
		"publishers/acme/books/dune/extra", "publishers/acme/books/\xff", "publishers/ü/books/日本",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		n, err := ParseBookName(in)
		if err != nil {
			if !errors.Is(err, ErrInvalidResourceName) {
				t.Fatalf("error does not wrap sentinel: %v", err)
			}
			return
		}
		if got := n.String(); got != in {
			t.Fatalf("String() = %q, want %q", got, in)
		}
		if err := n.ValidateAllowWildcard(); err != nil {
			t.Fatalf("accepted input fails ValidateAllowWildcard: %v", err)
		}
		if !utf8.ValidString(n.Publisher) || !utf8.ValidString(n.Book) {
			t.Fatalf("accepted invalid UTF-8: %q", in)
		}
		if strings.Count(in, "/") != 3 {
			t.Fatalf("accepted %q with %d slashes", in, strings.Count(in, "/"))
		}
		if (n.Publisher == "-" || n.Book == "-") != n.ContainsWildcard() {
			t.Fatalf("ContainsWildcard inconsistent for %q", in)
		}
	})
}
