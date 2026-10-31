package multi

import (
	"errors"
	"testing"
)

func TestParseBookNameDispatch(t *testing.T) {
	cases := map[string]BookName{
		"publishers/acme/books/dune": PublisherBookName{Publisher: "acme", Book: "dune"},
		"shelves/top/books/dune":     ShelfBookName{Shelf: "top", Book: "dune"},
	}
	for in, want := range cases {
		got, err := ParseBookName(in)
		if err != nil || got != want {
			t.Errorf("ParseBookName(%q) = %#v, %v; want %#v", in, got, err, want)
		}
		if got.String() != in || got.Type() != BookType {
			t.Errorf("String()/Type() of %q = %q/%q", in, got.String(), got.Type())
		}
		if err := got.Validate(); err != nil {
			t.Errorf("Validate() of %q = %v", in, err)
		}
	}
	_, err := ParseBookName("authors/x/books/dune")
	want := `invalid resource name: book name "authors/x/books/dune": matches no pattern of library.example.com/Book`
	if err == nil || err.Error() != want || !errors.Is(err, ErrInvalidResourceName) {
		t.Fatalf("error = %v", err)
	}
}

func TestVariants(t *testing.T) {
	p := MustParsePublisherBookName("publishers/acme/books/dune")
	if p.Parent() != (PublisherName{Publisher: "acme"}) {
		t.Errorf("Parent() = %+v", p.Parent())
	}
	s := MustParseShelfBookName("shelves/top/books/dune")
	if s.Parent() != (ShelfName{Shelf: "top"}) {
		t.Errorf("Parent() = %+v", s.Parent())
	}
	if _, err := ParsePublisherBookName("shelves/top/books/dune"); err == nil {
		t.Error("ParsePublisherBookName accepted a shelf pattern")
	}
	if BookNamePattern != PublisherBookNamePattern || ShelfBookNamePattern != "shelves/{shelf}/books/{book}" {
		t.Errorf("patterns: %q %q %q", BookNamePattern, PublisherBookNamePattern, ShelfBookNamePattern)
	}
	var iface BookName = ShelfBookName{Shelf: "-", Book: "x"}
	if !iface.ContainsWildcard() || iface.Validate() == nil || iface.ValidateAllowWildcard() != nil {
		t.Error("wildcard handling through the interface")
	}
	if _, err := iface.MarshalString(); err == nil {
		t.Error("MarshalString accepted a wildcard")
	}
}

func TestAccessors(t *testing.T) {
	req := &GetBookRequest{Name: "shelves/top/books/dune"}
	n, err := req.BookName()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := n.(ShelfBookName); !ok {
		t.Fatalf("BookName() = %#v", n)
	}
	book := &Book{Name: "publishers/acme/books/dune"}
	if _, ok := book.MustResourceName().(PublisherBookName); !ok {
		t.Fatalf("MustResourceName() = %#v", book.MustResourceName())
	}
}
