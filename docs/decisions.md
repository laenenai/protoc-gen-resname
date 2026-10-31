# Decisions

Design decisions for protoc-gen-resname, in the order they were taken. The
first block records what the owner decided before implementation started;
the second block records what was decided during implementation.

## Taken by the owner before implementation

1. **Greenfield.** §2 of the spec ("current state") is "none"; the only
   feature is `resname` and `features=` defaults to it.
2. **Open questions of §8 resolved:** (1) inline, no runtime package in v1 —
   an `ErrInvalidResourceName` sentinel per generated package; (2)
   permissive segment charset by default, `resname_strict_segments=true`
   enables `[a-z0-9-]{1,63}`; (3) short type names `BookName`,
   `ParseBookName`, `BookType`, `BookNamePattern`; (4) multi-pattern =
   interface `BookName` + variant structs `PublisherBookName`,
   `ShelfBookName`; (5) `(x *Book) ResourceName() (BookName, error)` plus
   `MustResourceName()`.
3. **Reference accessors only for name-like fields.** The `contracts`
   repository annotates bare-UUID `_id` fields in `domain/` with
   `resource_reference.type` as type information (a deliberate AIP-124
   deviation); resource names exist only in `api/`. Accessors are therefore
   generated only for `string` fields named `name`/`parent` or ending in
   `_name`; `resname_ref_fields=all` lifts the restriction.
4. **Request-wide checks.** Type uniqueness (§4.3) and the dangling
   reference rule (§4.6) span the whole `CodeGeneratorRequest`, not only
   the files with `Generate == true`.
5. **`paths=source_relative`** honoured like protoc-gen-go; output suffix
   `_resname.pb.go`.
6. **Golden scenarios** under `testdata/proto/<scenario>` with committed,
   buildable output under `testdata/golden/<scenario>`;
   `buf.build/googleapis/googleapis` provides `google/api/resource.proto`.
7. **Public repository, MIT licence** (owner correction received during
   setup; copyright 2026 Laenen AI).

## Taken during implementation

8. **One buf image for all scenarios.** `testdata/proto` is a single buf
   module built into one committed `image.binpb`; each test scenario selects
   the files under its directory and their transitive imports, so scenarios
   never see each other's types and `go test` needs no buf at test time.
   `mise run golden` rebuilds the image, the protoc-gen-go companions and the
   plugin's golden files.
9. **Golden packages live under `testdata/` and are named explicitly.** The
   go tool skips `testdata` in `./...`, so every mise task runs over
   `./... ./testdata/golden/...`. The table tests and `FuzzParseBookName`
   live in the golden packages themselves (`testdata/golden/<scenario>/name_test.go`).
10. **Feature construction sees the whole request.** Instead of the spec's
    bare `Generate(*protogen.Plugin, *protogen.File, Config)`, a feature is
    built once per request (`resname.New(gen, cfg, env)`), which is where
    the cross-file registry is validated, and then asked to `Generate(file)`
    per input file. Parameter parsing lives in `internal/plugin` so the
    in-process tests exercise it; `main.go` only wires `ParamFunc` and
    `--version`.
11. **Helpers once per Go package.** The sentinel `ErrInvalidResourceName`
    and the segment validator are emitted in the first emitting file (in
    request order) of each Go package. Generating a single file of a
    multi-file package in isolation (`buf generate --path`) therefore yields
    a package that does not compile until the first file is generated too —
    the same constraint protoc-gen-go has for cross-file message references.
12. **Validation scope.** Resource annotations (`google.api.resource`,
    `resource_definition`) are validated in *every* file of the request —
    errors in an imported file stop generation, since its resources feed
    parents and references. Style warnings (singular mismatch, missing
    plural, escaped identifiers) are reported only for generated files.
    `resource_reference` fields are validated only in generated files (an
    imported third-party file may reference types outside the request) but
    resolved against the whole request.
13. **Singular/plural warnings apply to message resources only.** A
    `resource_definition` usually describes a foreign type whose metadata
    the author cannot fix, so it gets no style warnings. The singular check
    skips singleton patterns (their last variable names the parent).
14. **Duplicate types.** Message vs message: error. A `resource_definition`
    whose type is already declared (by a message or an earlier definition):
    warning, first by file order wins, the definition is ignored.
15. **Reference validation after the field filter.** Under
    `resname_ref_fields=names`, an ineligible field (e.g. `order_id`) is
    skipped before its target is looked up, so a domain file may reference
    types that are not in the request. Eligible fields with an unknown
    target are errors; a `resource_reference` on a non-string field is an
    error; `type: "*"` / `child_type: "*"` generate nothing.
16. **Accessor names.** `name` → `<Kind>Name()`, `parent` → `ParentName()`
    (spec). protoc-gen-go exports every field as a struct field, so a
    `*_name` field cannot get a method of its own Go name: the `Name` suffix
    is replaced by `<Kind>Name` (`destination_name` referencing Publisher →
    `DestinationPublisherName()`), or by `ResourceName` when that would
    stutter (`customer_name` referencing Customer →
    `CustomerResourceName()`). Any other eligible field (only under `all`)
    gets a `Name` suffix (`order_id` → `OrderIdName()`). Repeated fields use
    the plural (`names` → `<Kind>Names()`); `names`, `parents` and `*_names`
    count as name-like. Residual collisions with message methods or fields
    get a `_` suffix and a warning.
17. **`child_type` targets must have known parents.** A `child_type`
    reference to a top-level resource, or to a resource whose parent
    pattern is not a known resource, is an error (there is nothing to type
    the accessor with). With one distinct parent the accessor returns that
    parent's struct; with several it returns the interface
    `<Kind>ParentName` (declared next to the child, with
    `Parse<Kind>ParentName` trying parents in the child's pattern order).
18. **Interfaces carry the full read-only surface.** The multi-pattern
    interface `BookName` and the parent interface `BookParentName` expose
    `String`, `MarshalString`, `Validate`, `ValidateAllowWildcard`,
    `ContainsWildcard` and `Type` (the spec listed the first three), so a
    caller holding the interface never needs a type switch for validation.
    `Parent()` stays on the variant structs because its return type differs
    per variant.
19. **`Parent()` returns the parent's variant struct.** A pattern's parent
    is the resource variant whose pattern equals the prefix with the last
    `collection/{variable}` (or trailing singleton literal) removed. When the
    parent is itself multi-pattern, `Parent()` returns the matching variant
    struct (e.g. `PublisherBookName`), not the interface; cross-package
    parents are imported through `QualifiedGoIdent`.
20. **Variant names.** Variables before the final one joined with the Kind
    (`publishers/{publisher}/books/{book}` → `PublisherBookName`); a
    singleton or single-variable pattern in a multi-pattern resource uses
    all its variables. Generated identifiers (`<Kind>Type`, `<Kind>Name`,
    `Parse…`, variant names, `<Kind>ParentName`) are checked for collisions
    with each other and with message names in the same Go package; a
    collision is an error naming both sides.
21. **Pattern variables colliding with method names** (`{type}`, `{string}`,
    `{parent}` …) become `Type_`, `String_`, … with a warning, as the spec
    prescribes for keywords and method names. Keywords cannot actually
    occur because fields are UpperCamel.
22. **Error messages.** Generation errors are `path:line:col: <element>:
    <message>` (`message Book`, `field GetBookRequest.name`,
    `resource_definition "type"`), the position taken from the request's
    source info and omitted when absent. Runtime errors are
    `invalid resource name: <display> name "<input>": <reason>` where
    `<display>` is `singular` or, if missing, the lowerCamel Kind; every one
    wraps the package sentinel with `%w`. Repeated accessors prefix the
    element index: `names[2]: invalid resource name: …`.
23. **Extra scenarios beyond the spec minimum:** `crosspkg` (resources and
    references across two Go packages in one request), `strict_segments`
    and `nowildcards` (parameter-dependent helper output), and the
    `warn/*` scenarios (every warning, each also run under `strict=true`).
24. **Dependency versions.** `google.golang.org/genproto/googleapis/api` is
    pinned to a 2026-08 revision because the latest one requires Go 1.26
    while the owner's toolchain pin is Go 1.25.6.
