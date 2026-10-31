package childtype

import (
	"errors"
	"testing"
)

func TestChildTypeOneParent(t *testing.T) {
	req := &ListMagazinesRequest{Parent: "publishers/acme"}
	p, err := req.ParentName()
	if err != nil || p != (PublisherName{Publisher: "acme"}) {
		t.Fatalf("ParentName() = %+v, %v", p, err)
	}
}

func TestChildTypeSeveralParents(t *testing.T) {
	cases := map[string]BookParentName{
		"publishers/acme": PublisherName{Publisher: "acme"},
		"shelves/top":     ShelfName{Shelf: "top"},
	}
	for in, want := range cases {
		got, err := (&ListBooksRequest{Parent: in}).ParentName()
		if err != nil || got != want {
			t.Errorf("ParentName() for %q = %#v, %v; want %#v", in, got, err, want)
		}
		if got.String() != in {
			t.Errorf("String() = %q", got.String())
		}
	}
	_, err := ParseBookParentName("authors/x")
	want := `invalid resource name: book parent name "authors/x": matches no parent pattern of library.example.com/Book`
	if err == nil || err.Error() != want || !errors.Is(err, ErrInvalidResourceName) {
		t.Fatalf("error = %v", err)
	}
	if MustParseBookParentName("shelves/top").Type() != ShelfType {
		t.Fatal("Type() of a shelf parent")
	}
}
