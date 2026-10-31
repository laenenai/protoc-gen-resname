// Package resname generates typed Go resource names from the
// google.api.resource annotations (spec §4).
package resname

// Values of Config.RefFields.
const (
	// RefFieldsNames generates resource_reference accessors only for string
	// fields named name, parent, names, parents or ending in _name / _names.
	RefFieldsNames = "names"
	// RefFieldsAll generates an accessor for every referencing string field.
	RefFieldsAll = "all"
)

// Config is the typed form of the resname_* and strict parameters.
type Config struct {
	Wildcards      bool   // resname_wildcards: ParseXName accepts "-" segments
	Refs           bool   // resname_refs: generate resource_reference accessors
	RefFields      string // resname_ref_fields: RefFieldsNames or RefFieldsAll
	StrictSegments bool   // resname_strict_segments: segments must match [a-z0-9-]{1,63}
	Strict         bool   // strict: warnings are errors
}

// Env is what the plugin provides besides the request.
type Env struct {
	Version string       // printed in the generated-file header
	Warn    func(string) // receives each warning (nil discards them)
}
