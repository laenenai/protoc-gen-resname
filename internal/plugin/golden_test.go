package plugin_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/laenenai/protoc-gen-resname/internal/plugin"
)

var update = flag.Bool("update", false, "rewrite the golden files from the plugin's current output")

const (
	imagePath = "../../testdata/proto/image.binpb"
	goldenDir = "../../testdata/golden"
)

// image loads the buf image built by `mise run golden` once per test binary.
var image = sync.OnceValues(func() (*descriptorpb.FileDescriptorSet, error) {
	raw, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, err
	}
	set := &descriptorpb.FileDescriptorSet{}
	// A buf image is a FileDescriptorSet plus buf-specific extension fields.
	if err := (proto.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, set); err != nil {
		return nil, err
	}
	return set, nil
})

// request builds a CodeGeneratorRequest for one scenario: every file under
// testdata/proto/<scenario>/ is generated and only those files plus their
// transitive imports are in the request, so scenarios never see each other.
func request(t *testing.T, scenario, params string) *pluginpb.CodeGeneratorRequest {
	t.Helper()
	set, err := image()
	if err != nil {
		t.Fatalf("load %s (run `mise run golden`): %v", imagePath, err)
	}
	byName := map[string]*descriptorpb.FileDescriptorProto{}
	for _, f := range set.File {
		byName[f.GetName()] = f
	}
	want := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		if want[name] {
			return
		}
		f, ok := byName[name]
		if !ok {
			t.Fatalf("scenario %s: %s not in image", scenario, name)
		}
		want[name] = true
		for _, dep := range f.Dependency {
			visit(dep)
		}
	}
	req := &pluginpb.CodeGeneratorRequest{Parameter: proto.String(params)}
	for _, f := range set.File {
		if strings.HasPrefix(f.GetName(), scenario+"/") {
			req.FileToGenerate = append(req.FileToGenerate, f.GetName())
			visit(f.GetName())
		}
	}
	if len(req.FileToGenerate) == 0 {
		t.Fatalf("scenario %s: no files under testdata/proto/%s", scenario, scenario)
	}
	for _, f := range set.File { // image order is topological
		if want[f.GetName()] {
			req.ProtoFile = append(req.ProtoFile, f)
		}
	}
	return req
}

// run executes the plugin in-process the way main does and returns the
// response files, the warnings written to stderr and the generation error.
func run(t *testing.T, req *pluginpb.CodeGeneratorRequest) (map[string]string, string, error) {
	t.Helper()
	cfg := plugin.DefaultConfig()
	var warnings bytes.Buffer
	opts := protogen.Options{ParamFunc: cfg.Set}
	gen, err := opts.New(req)
	if err != nil {
		return nil, "", err
	}
	err = plugin.Run(gen, cfg, plugin.Env{Version: "test", Warnings: &warnings})
	if err != nil {
		return nil, warnings.String(), err
	}
	resp := gen.Response()
	if resp.Error != nil {
		return nil, warnings.String(), fmt.Errorf("%s", resp.GetError())
	}
	files := map[string]string{}
	for _, f := range resp.File {
		files[f.GetName()] = f.GetContent()
	}
	return files, warnings.String(), nil
}

var goldenScenarios = []struct {
	name     string
	params   string
	warnings []string // substrings expected on stderr
}{
	{name: "single"},
	{name: "nested"},
	{name: "multi"},
	{name: "singleton"},
	{name: "external"},
	{name: "crosspkg"},
	{name: "childtype"},
	{name: "refids_names"},
	{name: "refids_all", params: "resname_ref_fields=all"},
	{name: "strict_segments", params: "resname_strict_segments=true"},
	{name: "nowildcards", params: "resname_wildcards=false"},
	{name: "warn/singular", warnings: []string{`warn/singular/book.proto:10:1: message Book: singular "book" does not match the last pattern variable {volume}`}},
	{name: "warn/plural", warnings: []string{`warn/plural/book.proto:10:1: message Book: plural is missing`}},
	{name: "warn/dupdef", warnings: []string{`warn/dupdef/b.proto:8:1: resource_definition "cloudresourcemanager.googleapis.com/Project": duplicates the one in warn/dupdef/a.proto; ignored`}},
}

func TestGolden(t *testing.T) {
	for _, sc := range goldenScenarios {
		t.Run(sc.name, func(t *testing.T) {
			params := "paths=source_relative"
			if sc.params != "" {
				params += "," + sc.params
			}
			files, warnings, err := run(t, request(t, sc.name, params))
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if len(files) == 0 {
				t.Fatalf("scenario emitted nothing")
			}
			for _, w := range sc.warnings {
				if !strings.Contains(warnings, w) {
					t.Errorf("missing warning %q in:\n%s", w, warnings)
				}
			}
			if len(sc.warnings) == 0 && warnings != "" {
				t.Errorf("unexpected warnings:\n%s", warnings)
			}
			// Every golden file on disk for this scenario must still be produced.
			onDisk := map[string]bool{}
			_ = filepath.WalkDir(filepath.Join(goldenDir, sc.name), func(p string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() && strings.HasSuffix(p, "_resname.pb.go") {
					rel, _ := filepath.Rel(goldenDir, p)
					onDisk[filepath.ToSlash(rel)] = true
				}
				return nil
			})
			for name, content := range files {
				if !strings.HasPrefix(name, sc.name+"/") {
					t.Errorf("file %s emitted outside the scenario directory", name)
				}
				path := filepath.Join(goldenDir, filepath.FromSlash(name))
				if *update {
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("missing golden %s (run `go test ./internal/... -update`): %v", path, err)
				}
				if string(want) != content {
					t.Errorf("%s differs from golden:\n%s", name, diff(string(want), content))
				}
				delete(onDisk, name)
			}
			if !*update {
				for name := range onDisk {
					t.Errorf("stale golden file %s is no longer generated", name)
				}
			}
		})
	}
}

// A file without resource annotations produces no output and no error.
func TestNoAnnotationsEmitsNothing(t *testing.T) {
	files, warnings, err := run(t, request(t, "noannot", "paths=source_relative"))
	if err != nil || len(files) != 0 || warnings != "" {
		t.Fatalf("files=%v warnings=%q err=%v; want nothing", files, warnings, err)
	}
}

// Every validation rule of spec §4.3 / §4.6 has a scenario with a stable
// error message.
func TestInvalid(t *testing.T) {
	cases := map[string]string{
		"invalid/alternate":            `invalid/alternate/bad.proto:10:1: message Book: pattern "publishers/{publisher}/{book}": segment 3 ({book}) breaks the literal/{variable} alternation`,
		"invalid/leading_variable":     `invalid/leading_variable/bad.proto:10:1: message Book: pattern "{publisher}/books/{book}": must start with a literal collection id`,
		"invalid/dupvar":               `invalid/dupvar/bad.proto:10:1: message Book: pattern "publishers/{publisher}/books/{publisher}": variable {publisher} appears twice`,
		"invalid/literal":              `invalid/literal/bad.proto:10:1: message Book: pattern "Publishers/{publisher}/books/{book}": segment 1 (Publishers) is not a collection id ([a-z][a-zA-Z0-9]*)`,
		"invalid/variable":             `invalid/variable/bad.proto:10:1: message Book: pattern "publishers/{Publisher}/books/{book}": segment 2 ({Publisher}) is not a variable ({snake_name})`,
		"invalid/no_pattern":           `invalid/no_pattern/bad.proto:10:1: message Book: resource has no pattern`,
		"invalid/type_format":          `invalid/type_format/bad.proto:10:1: message Book: type "library.example.com/book" must be {service}/{Kind} with an UpperCamel Kind`,
		"invalid/type_no_service":      `invalid/type_no_service/bad.proto:10:1: message Book: type "Book" must be {service}/{Kind} with an UpperCamel Kind`,
		"invalid/type_dup":             `invalid/type_dup/bad.proto:12:1: message Book: type "library.example.com/Book" is already declared by message Other in invalid/type_dup_other/other.proto`,
		"invalid/name_field_missing":   `invalid/name_field_missing/bad.proto:10:1: message Book: name_field "name" does not exist on the message`,
		"invalid/name_field_type":      `invalid/name_field_type/bad.proto:10:1: message Book: name_field "id" must be a string, got int64`,
		"invalid/ref_unknown":          `invalid/ref_unknown/bad.proto:11:3: field GetBookRequest.name: resource_reference type "library.example.com/Book" is not declared anywhere in the request`,
		"invalid/child_ref_unknown":    `invalid/child_ref_unknown/bad.proto:11:3: field ListBooksRequest.parent: resource_reference child_type "library.example.com/Book" is not declared anywhere in the request`,
		"invalid/child_top_level":      `invalid/child_top_level/bad.proto:22:3: field ListPublishersRequest.parent: child_type "library.example.com/Publisher" has no parent (pattern "publishers/{publisher}" is top-level)`,
		"invalid/child_parent_unknown": `invalid/child_parent_unknown/bad.proto:22:3: field ListBooksRequest.parent: child_type "library.example.com/Book": parent pattern "publishers/{publisher}" is not a known resource`,
		"invalid/ref_not_string":       `invalid/ref_not_string/bad.proto:22:3: field GetBookRequest.name: resource_reference requires a string field, got int64`,
	}
	for scenario, want := range cases {
		t.Run(scenario, func(t *testing.T) {
			_, _, err := run(t, request(t, scenario, "paths=source_relative"))
			if err == nil || err.Error() != want {
				t.Fatalf("error = %v\n   want %s", err, want)
			}
		})
	}
}

// With strict=true every warning is an error.
func TestStrictTurnsWarningsIntoErrors(t *testing.T) {
	for _, sc := range goldenScenarios {
		if len(sc.warnings) == 0 {
			continue
		}
		t.Run(sc.name, func(t *testing.T) {
			_, _, err := run(t, request(t, sc.name, "paths=source_relative,strict=true"))
			if err == nil || err.Error() != sc.warnings[0] {
				t.Fatalf("error = %v\n   want %s", err, sc.warnings[0])
			}
		})
	}
}

func TestUnknownParameter(t *testing.T) {
	_, _, err := run(t, request(t, "single", "bogus=1"))
	if err == nil || err.Error() != `unknown parameter "bogus"` {
		t.Fatalf("error = %v", err)
	}
	_, _, err = run(t, request(t, "single", "features=nope"))
	if err == nil || err.Error() != `unknown feature "nope"` {
		t.Fatalf("error = %v", err)
	}
	_, _, err = run(t, request(t, "single", "resname_ref_fields=some"))
	if err == nil || err.Error() != `resname_ref_fields must be "names" or "all", got "some"` {
		t.Fatalf("error = %v", err)
	}
}

// The same request always produces byte-identical output.
func TestDeterministic(t *testing.T) {
	for _, sc := range goldenScenarios {
		params := "paths=source_relative"
		if sc.params != "" {
			params += "," + sc.params
		}
		first, _, err := run(t, request(t, sc.name, params))
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			again, _, err := run(t, request(t, sc.name, params))
			if err != nil {
				t.Fatal(err)
			}
			if len(again) != len(first) {
				t.Fatalf("%s: run %d emitted %d files, first run %d", sc.name, i, len(again), len(first))
			}
			for name, content := range first {
				if again[name] != content {
					t.Fatalf("%s: %s differs between runs", sc.name, name)
				}
			}
		}
	}
}

// paths=import (the protoc-gen-go default) places files by go_package.
func TestPathsImport(t *testing.T) {
	files, _, err := run(t, request(t, "single", ""))
	if err != nil {
		t.Fatal(err)
	}
	for name := range files {
		if name != "github.com/laenenai/protoc-gen-resname/testdata/golden/single/library_resname.pb.go" {
			t.Fatalf("unexpected output path %s", name)
		}
	}
}

// The header names the plugin, its version and the source file.
func TestHeader(t *testing.T) {
	files, _, err := run(t, request(t, "single", "paths=source_relative"))
	if err != nil {
		t.Fatal(err)
	}
	content := files["single/library_resname.pb.go"]
	for _, want := range []string{
		"// Code generated by protoc-gen-resname test. DO NOT EDIT.\n",
		"// source: single/library.proto\n",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("header lacks %q:\n%s", want, content)
		}
	}
}

// diff renders a minimal line-based difference for test output.
func diff(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	n := len(wl)
	if len(gl) > n {
		n = len(gl)
	}
	shown := 0
	for i := 0; i < n && shown < 40; i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			fmt.Fprintf(&b, "%4d: -%s\n      +%s\n", i+1, w, g)
			shown++
		}
	}
	return b.String()
}
