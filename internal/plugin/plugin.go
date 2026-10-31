// Package plugin holds the feature registry, the typed plugin parameters and
// the per-file dispatch loop shared by every feature.
package plugin

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/laenenai/protoc-gen-resname/internal/feature/resname"
)

// Config is the typed form of the plugin parameters (spec §3.3).
type Config struct {
	Features []string
	Resname  resname.Config
}

// DefaultConfig returns the parameter defaults: features=resname,
// resname_wildcards=true, resname_refs=true, resname_ref_fields=names,
// resname_strict_segments=false, strict=false.
func DefaultConfig() Config {
	return Config{
		Features: []string{"resname"},
		Resname: resname.Config{
			Wildcards: true,
			Refs:      true,
			RefFields: resname.RefFieldsNames,
		},
	}
}

// Set applies one name=value parameter; it is the protogen.Options.ParamFunc.
// protogen handles paths=, module= and M<file>=<path> itself before calling
// it.
func (c *Config) Set(name, value string) error {
	switch name {
	case "features":
		c.Features = nil
		for _, f := range strings.Split(value, "+") {
			if f = strings.TrimSpace(f); f != "" {
				c.Features = append(c.Features, f)
			}
		}
		if len(c.Features) == 0 {
			return fmt.Errorf("features must name at least one feature")
		}
	case "resname_wildcards":
		return setBool(name, value, &c.Resname.Wildcards)
	case "resname_refs":
		return setBool(name, value, &c.Resname.Refs)
	case "resname_ref_fields":
		switch value {
		case resname.RefFieldsNames, resname.RefFieldsAll:
			c.Resname.RefFields = value
		default:
			return fmt.Errorf("resname_ref_fields must be %q or %q, got %q", resname.RefFieldsNames, resname.RefFieldsAll, value)
		}
	case "resname_strict_segments":
		return setBool(name, value, &c.Resname.StrictSegments)
	case "strict":
		return setBool(name, value, &c.Resname.Strict)
	default:
		return fmt.Errorf("unknown parameter %q", name)
	}
	return nil
}

func setBool(name, value string, dst *bool) error {
	if value == "" {
		*dst = true
		return nil
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("%s must be true or false, got %q", name, value)
	}
	*dst = b
	return nil
}

// Env is what the entry point provides to the features besides the request.
type Env struct {
	Version  string    // printed in the generated-file header
	Warnings io.Writer // where warnings go; nil means os.Stderr
}

// Feature is one generator: it sees the whole request when constructed (for
// cross-file registries and validation) and is then asked to emit one output
// file per input file.
type Feature interface {
	Generate(f *protogen.File) error
}

// Constructor builds a feature for one request.
type Constructor func(gen *protogen.Plugin, cfg Config, env Env) (Feature, error)

// Registry maps feature names to constructors.
var Registry = map[string]Constructor{
	"resname": func(gen *protogen.Plugin, cfg Config, env Env) (Feature, error) {
		return resname.New(gen, cfg.Resname, resname.Env{Version: env.Version, Warn: warnFunc(env)})
	},
}

func warnFunc(env Env) func(string) {
	w := env.Warnings
	if w == nil {
		w = os.Stderr
	}
	return func(msg string) { fmt.Fprintf(w, "protoc-gen-resname: warning: %s\n", msg) }
}

// Run executes every enabled feature over every file to generate.
func Run(gen *protogen.Plugin, cfg Config, env Env) error {
	var features []Feature
	seen := map[string]bool{}
	for _, name := range cfg.Features {
		if seen[name] {
			continue
		}
		seen[name] = true
		ctor, ok := Registry[name]
		if !ok {
			return fmt.Errorf("unknown feature %q", name)
		}
		f, err := ctor(gen, cfg, env)
		if err != nil {
			return err
		}
		features = append(features, f)
	}
	for _, file := range gen.Files {
		if !file.Generate {
			continue
		}
		for _, f := range features {
			if err := f.Generate(file); err != nil {
				return err
			}
		}
	}
	return nil
}
