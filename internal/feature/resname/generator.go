package resname

import "google.golang.org/protobuf/compiler/protogen"

// Generator emits one <file>_resname.pb.go per input file that declares
// resources or references them.
type Generator struct {
	gen *protogen.Plugin
	cfg Config
	env Env
	m   *model
}

// New validates every resource annotation in the request (all files, not
// only those being generated, so cross-file references and the type
// uniqueness rule see everything) and returns a generator for it.
func New(gen *protogen.Plugin, cfg Config, env Env) (*Generator, error) {
	m, err := build(gen, cfg, env)
	if err != nil {
		return nil, err
	}
	return &Generator{gen: gen, cfg: cfg, env: env, m: m}, nil
}

// Generate emits the file for f, or nothing when f declares no resource and
// references none.
func (g *Generator) Generate(f *protogen.File) error {
	fm := g.m.files[f]
	if fm == nil || fm.empty() {
		return nil
	}
	out := g.gen.NewGeneratedFile(f.GeneratedFilenamePrefix+"_resname.pb.go", f.GoImportPath)
	emit(out, f, fm, g.cfg, g.env.Version)
	return nil
}
