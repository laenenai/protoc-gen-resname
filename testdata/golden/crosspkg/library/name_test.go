package library

import (
	"testing"

	"github.com/laenenai/protoc-gen-resname/testdata/golden/crosspkg/common"
)

func TestParentAcrossPackages(t *testing.T) {
	n := MustParseBookName("projects/p1/locations/eu/books/dune")
	want := common.LocationName{Project: "p1", Location: "eu"}
	if n.Parent() != want {
		t.Fatalf("Parent() = %+v", n.Parent())
	}
	if n.Parent().Parent() != (common.ProjectName{Project: "p1"}) {
		t.Fatalf("Parent().Parent() = %+v", n.Parent().Parent())
	}
	p, err := (&ListBooksRequest{Parent: "projects/p1/locations/eu"}).ParentName()
	if err != nil || p != want {
		t.Fatalf("ParentName() = %+v, %v", p, err)
	}
	l, err := (&GetLocationRequest{Name: "projects/p1/locations/eu"}).LocationName()
	if err != nil || l != want {
		t.Fatalf("LocationName() = %+v, %v", l, err)
	}
}
