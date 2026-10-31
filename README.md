# protoc-gen-resname

A `protoc`/`buf` plugin that generates **typed Go resource names** from the
standard `google/api/resource.proto` annotations (AIP-122 / AIP-123). It runs
next to `protoc-gen-go` and emits one `<file>_resname.pb.go` per input file
that declares resources.

- Generated code depends only on the Go standard library and
  `google.golang.org/protobuf` — no runtime package of its own.
- Deterministic: the same input always produces byte-identical output.
- Transport-agnostic: nothing in the output knows about gRPC or Connect.

See [`docs/spec.md`](docs/spec.md) for the full specification and
[`docs/decisions.md`](docs/decisions.md) for the design decisions.

## Install

```sh
go install github.com/laenenai/protoc-gen-resname/cmd/protoc-gen-resname@latest
```

## Usage with buf

`buf.gen.yaml`:

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt: [paths=source_relative]
  - local: protoc-gen-resname
    out: gen
    opt: [paths=source_relative, features=resname]
```

Your proto needs `buf.build/googleapis/googleapis` as a dependency for
`google/api/resource.proto`.

### Parameters

| Parameter | Default | Meaning |
|---|---|---|
| `features` | `resname` | Enabled features, joined with `+` |
| `paths` | `import` | `import` or `source_relative`, as in protoc-gen-go |
| `resname_wildcards` | `true` | `ParseXName` accepts `-` wildcard segments |
| `resname_refs` | `true` | Generate accessors for `resource_reference` fields |
| `resname_ref_fields` | `names` | `names`: only `name`, `parent` and `*_name` fields get accessors; `all`: every string field with a reference |
| `resname_strict_segments` | `false` | Segments must match `[a-z0-9-]{1,63}` instead of "non-empty, no `/`, valid UTF-8" |
| `strict` | `false` | Warnings become errors |

## Example

```proto
syntax = "proto3";
package library.v1;

import "google/api/resource.proto";

message Book {
  option (google.api.resource) = {
    type: "library.example.com/Book"
    pattern: "publishers/{publisher}/books/{book}"
    singular: "book"
    plural: "books"
  };
  string name = 1;
}

message GetBookRequest {
  string name = 1 [(google.api.resource_reference).type = "library.example.com/Book"];
}
```

generates, in `library_resname.pb.go`:

```go
const BookType = "library.example.com/Book"
const BookNamePattern = "publishers/{publisher}/books/{book}"

type BookName struct {
	Publisher string
	Book      string
}

func ParseBookName(name string) (BookName, error)
func MustParseBookName(name string) BookName
func (n BookName) String() string
func (n BookName) MarshalString() (string, error)
func (n *BookName) UnmarshalString(s string) error
func (n BookName) Validate() error
func (n BookName) ValidateAllowWildcard() error
func (n BookName) ContainsWildcard() bool
func (n BookName) Type() string
func (n BookName) IsZero() bool
func (n BookName) Parent() PublisherName // when Publisher is a known resource

func (x *Book) ResourceName() (BookName, error)
func (x *Book) MustResourceName() BookName
func (x *GetBookRequest) BookName() (BookName, error)
```

and a per-package sentinel `ErrInvalidResourceName` that every parse and
validation error wraps:

```go
n, err := library.ParseBookName("publishers/acme/books/dune")
if errors.Is(err, library.ErrInvalidResourceName) { /* ... */ }
fmt.Println(n.Publisher, n.Book) // acme dune
fmt.Println(n.Parent())          // publishers/acme
```

### Reference accessors and bare-id fields

Accessors for `google.api.resource_reference` are generated only for string
fields named `name` or `parent`, or ending in `_name`. Fields ending in `_id`
get none: in this organisation's contracts, bare-UUID `_id` fields in domain
messages carry a `resource_reference.type` purely as type information, and
only API messages carry real resource names. Pass
`resname_ref_fields=all` to generate an accessor for every referencing
string field.

Accessor names: `name` → `<Kind>Name()`, `parent` → `ParentName()`
(`child_type` references return the parent's name type, or a
`<Kind>ParentName` interface when several parents are possible). Because
protoc-gen-go exports every field as a struct field, a `*_name` field gets
its suffix replaced by the Kind (`destination_name` referencing Publisher →
`DestinationPublisherName()`), or `<Kind>ResourceName()` when that would
stutter. Repeated fields use the plural (`names` → `<Kind>Names()`).

## Development

```sh
mise install     # pinned go, buf, protoc-gen-go, staticcheck
mise run ci      # build, vet, lint, test
mise run golden  # rebuild testdata images and regenerate golden files
mise run fuzz    # 60 s of FuzzParseBookName
```

## Licence

MIT — see [LICENSE](LICENSE).
