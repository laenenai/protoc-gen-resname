package resname

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/laenenai/protoc-gen-resname/internal/annotations"
	"github.com/laenenai/protoc-gen-resname/internal/naming"
)

// Resource is one google.api.resource message or resource_definition after
// validation. Everything the emitter needs is resolved here; template.go
// never touches descriptors.
type Resource struct {
	Type      string // "library.example.com/Book"
	Kind      string // "Book"
	Display   string // name used in error messages: singular, else lowerCamel Kind
	Singular  string
	Plural    string
	NameField string
	Patterns  []*Pattern
	Variants  []*Variant // one per pattern, in declaration order; the first is canonical
	Multi     bool       // more than one pattern: NameIdent is an interface

	File    *protogen.File    // declaring file
	Message *protogen.Message // nil for a resource_definition

	NameIdent   protogen.GoIdent // BookName
	Parents     []*Variant       // distinct known parents across the patterns, in pattern order
	ParentIdent protogen.GoIdent // BookParentName, set when len(Parents) > 1

	nameFieldGoName string // Go name of the name field on Message
	where           string // diagnostics prefix: "file:line:col: message Book"
}

// Variant is one pattern of a resource together with its Go struct type.
type Variant struct {
	Resource *Resource
	Pattern  *Pattern
	Ident    protogen.GoIdent // BookName (single pattern) or PublisherBookName
	Fields   []Field          // one per pattern variable
	Parent   *Variant         // the resource variant whose pattern is this pattern's prefix; nil if unknown or top-level
}

// Field is one struct field of a Variant.
type Field struct {
	Variable string // pattern variable, e.g. "key_ring"
	GoName   string // "KeyRing"
}

// ParseIdent is the Parse<Variant> function.
func (v *Variant) ParseIdent() protogen.GoIdent {
	return protogen.GoIdent{GoName: "Parse" + v.Ident.GoName, GoImportPath: v.Ident.GoImportPath}
}

// Field returns the struct field for a pattern variable.
func (v *Variant) Field(variable string) Field {
	for _, f := range v.Fields {
		if f.Variable == variable {
			return f
		}
	}
	return Field{}
}

// Reference is one resource_reference accessor to emit on a message.
type Reference struct {
	Message  *protogen.Message
	Field    *protogen.Field
	Method   string           // BookName, ParentName, DestinationName, ...
	Return   protogen.GoIdent // element type returned
	Parse    protogen.GoIdent // function parsing one string into Return
	Repeated bool
}

// fileModel is everything emitted for one input file.
type fileModel struct {
	Resources []*Resource
	Refs      []*Reference
	Helpers   bool // this file carries the package's sentinel and helpers
	defs      []*Resource
	msgs      []*Resource
}

func (fm *fileModel) empty() bool { return len(fm.Resources) == 0 && len(fm.Refs) == 0 }

// model is the request-wide registry built once per run.
type model struct {
	cfg       Config
	env       Env
	byType    map[string]*Resource
	byPattern map[string]*Variant // first variant declared with each raw pattern
	idents    map[protogen.GoImportPath]map[string]string
	files     map[*protogen.File]*fileModel
}

func build(gen *protogen.Plugin, cfg Config, env Env) (*model, error) {
	m := &model{
		cfg:       cfg,
		env:       env,
		byType:    map[string]*Resource{},
		byPattern: map[string]*Variant{},
		idents:    map[protogen.GoImportPath]map[string]string{},
		files:     map[*protogen.File]*fileModel{},
	}
	for _, f := range gen.Files {
		m.files[f] = &fileModel{}
		for _, msg := range f.Messages {
			if err := m.reserveMessageIdents(f, msg); err != nil {
				return nil, err
			}
		}
	}
	// Messages first: a message resource always wins over a
	// resource_definition of the same type.
	for _, f := range gen.Files {
		for _, msg := range f.Messages {
			if err := m.addMessageResources(f, msg); err != nil {
				return nil, err
			}
		}
	}
	for _, f := range gen.Files {
		for i, rd := range annotations.ResourceDefinitions(f.Desc) {
			where := definitionWhere(f, i, rd.GetType())
			if prev, ok := m.byType[rd.GetType()]; ok {
				if err := m.warn(f, fmt.Sprintf("%s: duplicates the one in %s; ignored", where, prev.File.Desc.Path())); err != nil {
					return nil, err
				}
				continue
			}
			r, err := m.newResource(f, nil, rd, where)
			if err != nil {
				return nil, err
			}
			m.files[f].defs = append(m.files[f].defs, r)
		}
	}
	for _, f := range gen.Files {
		fm := m.files[f]
		fm.Resources = append(append(fm.Resources, fm.defs...), fm.msgs...)
	}
	m.resolveParents(gen)
	if cfg.Refs {
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			for _, msg := range f.Messages {
				if err := m.addReferences(f, msg); err != nil {
					return nil, err
				}
			}
		}
	}
	// The first emitting file of each Go package carries the shared helpers.
	helpers := map[protogen.GoImportPath]bool{}
	for _, f := range gen.Files {
		fm := m.files[f]
		if !f.Generate || fm.empty() || helpers[f.GoImportPath] {
			continue
		}
		helpers[f.GoImportPath] = true
		fm.Helpers = true
	}
	return m, nil
}

// warn reports a style issue for a generated file: to Env.Warn normally, as
// an error under strict. Issues in files that are not generated (imported
// dependencies) are not reported.
func (m *model) warn(f *protogen.File, msg string) error {
	if !f.Generate {
		return nil
	}
	if m.cfg.Strict {
		return fmt.Errorf("%s", msg)
	}
	if m.env.Warn != nil {
		m.env.Warn(msg)
	}
	return nil
}

func (m *model) reserveMessageIdents(f *protogen.File, msg *protogen.Message) error {
	if err := m.reserveIdent(f.GoImportPath, msg.GoIdent.GoName, "message "+messageName(f, msg)+" in "+f.Desc.Path()); err != nil {
		return err
	}
	for _, nested := range msg.Messages {
		if err := m.reserveMessageIdents(f, nested); err != nil {
			return err
		}
	}
	return nil
}

func (m *model) reserveIdent(pkg protogen.GoImportPath, name, owner string) error {
	if m.idents[pkg] == nil {
		m.idents[pkg] = map[string]string{}
	}
	if prev, ok := m.idents[pkg][name]; ok {
		return fmt.Errorf("generated identifier %s collides with %s", name, prev)
	}
	m.idents[pkg][name] = owner
	return nil
}

func (m *model) addMessageResources(f *protogen.File, msg *protogen.Message) error {
	if rd := annotations.Resource(msg.Desc); rd != nil {
		r, err := m.newResource(f, msg, rd, where(f, msg.Desc)+": message "+messageName(f, msg))
		if err != nil {
			return err
		}
		m.files[f].msgs = append(m.files[f].msgs, r)
	}
	for _, nested := range msg.Messages {
		if err := m.addMessageResources(f, nested); err != nil {
			return err
		}
	}
	return nil
}

// newResource validates one annotation (spec §4.3) and registers the
// resource and its Go identifiers.
func (m *model) newResource(f *protogen.File, msg *protogen.Message, rd resourceDescriptor, where string) (*Resource, error) {
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%s: %s", where, fmt.Sprintf(format, args...))
	}
	kind, err := parseType(rd.GetType())
	if err != nil {
		return nil, fail("%v", err)
	}
	if prev, ok := m.byType[rd.GetType()]; ok {
		owner := "resource_definition"
		if prev.Message != nil {
			owner = "message " + messageName(prev.File, prev.Message)
		}
		return nil, fail("type %q is already declared by %s in %s", rd.GetType(), owner, prev.File.Desc.Path())
	}
	if len(rd.GetPattern()) == 0 {
		return nil, fail("resource has no pattern")
	}
	r := &Resource{
		Type:      rd.GetType(),
		Kind:      kind,
		Singular:  rd.GetSingular(),
		Plural:    rd.GetPlural(),
		NameField: rd.GetNameField(),
		File:      f,
		Message:   msg,
		NameIdent: f.GoImportPath.Ident(kind + "Name"),
		where:     where,
	}
	if r.NameField == "" {
		r.NameField = "name"
	}
	r.Display = r.Singular
	if r.Display == "" {
		r.Display = naming.LowerFirst(kind)
	}
	for _, raw := range rd.GetPattern() {
		p, err := parsePattern(raw)
		if err != nil {
			return nil, fail("%v", err)
		}
		r.Patterns = append(r.Patterns, p)
	}
	r.Multi = len(r.Patterns) > 1
	if msg != nil {
		var nameField *protogen.Field
		for _, fld := range msg.Fields {
			if string(fld.Desc.Name()) == r.NameField {
				nameField = fld
				break
			}
		}
		if nameField == nil {
			return nil, fail("name_field %q does not exist on the message", r.NameField)
		}
		if nameField.Desc.Kind() != protoreflect.StringKind || nameField.Desc.IsList() {
			return nil, fail("name_field %q must be a string, got %s", r.NameField, fieldKind(nameField))
		}
		r.nameFieldGoName = nameField.GoName
	}
	// Go identifiers: Kind-derived names must be unique in the package.
	for _, name := range []string{kind + "Type", kind + "NamePattern", kind + "Name", "Parse" + kind + "Name", "MustParse" + kind + "Name"} {
		if err := m.reserveIdent(f.GoImportPath, name, "resource "+r.Type+" in "+f.Desc.Path()); err != nil {
			return nil, fail("%v", err)
		}
	}
	for _, p := range r.Patterns {
		v := &Variant{Resource: r, Pattern: p, Ident: r.NameIdent}
		if r.Multi {
			v.Ident = f.GoImportPath.Ident(variantName(p, kind))
			for _, name := range []string{v.Ident.GoName, v.Ident.GoName + "Pattern", "Parse" + v.Ident.GoName, "MustParse" + v.Ident.GoName} {
				if err := m.reserveIdent(f.GoImportPath, name, "pattern "+p.Raw+" of resource "+r.Type+" in "+f.Desc.Path()); err != nil {
					return nil, fail("%v", err)
				}
			}
		}
		for _, variable := range p.Variables {
			goName, escaped := naming.Escape(naming.UpperCamel(variable), reservedFieldNames)
			if escaped {
				if err := m.warn(f, fmt.Sprintf("%s: pattern variable {%s} is named %s in Go to avoid a method name", where, variable, goName)); err != nil {
					return nil, err
				}
			}
			v.Fields = append(v.Fields, Field{Variable: variable, GoName: goName})
		}
		r.Variants = append(r.Variants, v)
		if _, ok := m.byPattern[p.Raw]; !ok {
			m.byPattern[p.Raw] = v
		}
	}
	m.byType[r.Type] = r
	// Style warnings (errors under strict) for message resources only: a
	// resource_definition usually describes a foreign type whose metadata
	// the author cannot fix.
	if msg == nil {
		return r, nil
	}
	for _, p := range r.Patterns {
		if p.Singleton || r.Singular == "" {
			continue
		}
		last := p.Variables[len(p.Variables)-1]
		if naming.UpperCamel(last) != naming.UpperCamel(r.Singular) {
			if err := m.warn(f, fmt.Sprintf("%s: singular %q does not match the last pattern variable {%s}", where, r.Singular, last)); err != nil {
				return nil, err
			}
		}
	}
	if r.Plural == "" {
		if err := m.warn(f, where+": plural is missing"); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// resourceDescriptor is the subset of annotations.ResourceDescriptor the
// model reads.
type resourceDescriptor interface {
	GetType() string
	GetPattern() []string
	GetNameField() string
	GetSingular() string
	GetPlural() string
}

// reservedFieldNames are the methods of every generated name type; a pattern
// variable mapping onto one gets an underscore suffix.
var reservedFieldNames = map[string]bool{
	"String": true, "MarshalString": true, "UnmarshalString": true, "Validate": true,
	"ValidateAllowWildcard": true, "ContainsWildcard": true, "Type": true, "Parent": true, "IsZero": true,
}

// variantName joins the variables before the final one with the Kind
// (spec §4.5): publishers/{publisher}/books/{book} → PublisherBookName. A
// singleton or a pattern with a single variable uses all its variables.
func variantName(p *Pattern, kind string) string {
	vars := p.Variables
	if !p.Singleton && len(vars) > 1 {
		vars = vars[:len(vars)-1]
	}
	var b strings.Builder
	for _, v := range vars {
		b.WriteString(naming.UpperCamel(v))
	}
	b.WriteString(kind)
	b.WriteString("Name")
	return b.String()
}

// resolveParents links every variant to the variant owning its prefix
// pattern and derives the per-resource parent set.
func (m *model) resolveParents(gen *protogen.Plugin) {
	for _, f := range gen.Files {
		for _, r := range m.files[f].Resources {
			for _, v := range r.Variants {
				if v.Pattern.ParentRaw == "" {
					continue
				}
				if parent, ok := m.byPattern[v.Pattern.ParentRaw]; ok {
					v.Parent = parent
					if !containsVariant(r.Parents, parent) {
						r.Parents = append(r.Parents, parent)
					}
				}
			}
			if len(r.Parents) > 1 {
				r.ParentIdent = r.File.GoImportPath.Ident(r.Kind + "ParentName")
			}
		}
	}
}

func containsVariant(vs []*Variant, v *Variant) bool {
	for _, x := range vs {
		if x == v {
			return true
		}
	}
	return false
}

// addReferences builds the resource_reference accessors of a message and
// its nested messages (spec §4.6).
func (m *model) addReferences(f *protogen.File, msg *protogen.Message) error {
	taken := map[string]bool{"Reset": true, "String": true, "ProtoMessage": true, "ProtoReflect": true, "Descriptor": true}
	if annotations.Resource(msg.Desc) != nil {
		taken["ResourceName"], taken["MustResourceName"] = true, true
	}
	for _, fld := range msg.Fields {
		taken[fld.GoName] = true // exported struct field under the open API
		for _, prefix := range []string{"Get", "Set", "Has", "Clear"} {
			taken[prefix+fld.GoName] = true
		}
	}
	for _, fld := range msg.Fields {
		ann := annotations.ResourceReference(fld.Desc)
		if ann == nil {
			continue
		}
		typ, child := ann.GetType(), ann.GetChildType()
		if typ == "*" || child == "*" || typ == "" && child == "" {
			continue
		}
		if !m.eligible(fld) {
			continue
		}
		where := where(f, fld.Desc) + ": field " + messageName(f, msg) + "." + string(fld.Desc.Name())
		if fld.Desc.Kind() != protoreflect.StringKind {
			return fmt.Errorf("%s: resource_reference requires a string field, got %s", where, fieldKind(fld))
		}
		ref := &Reference{Message: msg, Field: fld, Repeated: fld.Desc.IsList()}
		if child == "" {
			target, ok := m.byType[typ]
			if !ok {
				return fmt.Errorf("%s: resource_reference type %q is not declared anywhere in the request", where, typ)
			}
			ref.Method = methodName(fld, target.Kind, false)
			ref.Return = target.NameIdent
			ref.Parse = protogen.GoIdent{GoName: "Parse" + target.NameIdent.GoName, GoImportPath: target.NameIdent.GoImportPath}
		} else {
			target, ok := m.byType[child]
			if !ok {
				return fmt.Errorf("%s: resource_reference child_type %q is not declared anywhere in the request", where, child)
			}
			for _, v := range target.Variants {
				if v.Pattern.ParentRaw == "" {
					return fmt.Errorf("%s: child_type %q has no parent (pattern %q is top-level)", where, child, v.Pattern.Raw)
				}
				if v.Parent == nil {
					return fmt.Errorf("%s: child_type %q: parent pattern %q is not a known resource", where, child, v.Pattern.ParentRaw)
				}
			}
			ref.Method = methodName(fld, target.Kind, true)
			if len(target.Parents) == 1 {
				ref.Return = target.Parents[0].Ident
				ref.Parse = target.Parents[0].ParseIdent()
			} else {
				ref.Return = target.ParentIdent
				ref.Parse = protogen.GoIdent{GoName: "Parse" + target.ParentIdent.GoName, GoImportPath: target.ParentIdent.GoImportPath}
			}
		}
		if name, escaped := naming.Escape(ref.Method, taken); escaped {
			if err := m.warn(f, fmt.Sprintf("%s: accessor %s renamed to %s to avoid a method of the same name", where, ref.Method, name)); err != nil {
				return err
			}
			ref.Method = name
		}
		taken[ref.Method] = true
		m.files[f].Refs = append(m.files[f].Refs, ref)
	}
	for _, nested := range msg.Messages {
		if err := m.addReferences(f, nested); err != nil {
			return err
		}
	}
	return nil
}

// eligible applies resname_ref_fields: under "names" only name, parent,
// names, parents and *_name / *_names fields get accessors.
func (m *model) eligible(fld *protogen.Field) bool {
	if m.cfg.RefFields == RefFieldsAll {
		return true
	}
	n := string(fld.Desc.Name())
	switch n {
	case "name", "names", "parent", "parents":
		return true
	}
	return strings.HasSuffix(n, "_name") || strings.HasSuffix(n, "_names")
}

// methodName picks the accessor name. name → <Kind>Name and parent →
// ParentName as in spec §4.6; a field ending in _name has that suffix
// replaced by <Kind>Name (destination_name → DestinationPublisherName), or
// becomes <Kind>ResourceName when that would stutter (customer_name →
// CustomerResourceName); any other field gets a Name suffix. Repeated fields
// take the plural forms. The field's own Go name is never reused because
// protoc-gen-go exports it as a struct field.
func methodName(fld *protogen.Field, kind string, child bool) string {
	suffix := "Name"
	if fld.Desc.IsList() {
		suffix = "Names"
	}
	if child {
		kind = "Parent"
	}
	switch string(fld.Desc.Name()) {
	case "parent", "parents":
		return "Parent" + suffix
	case "name", "names":
		return kind + suffix
	}
	goName := fld.GoName
	stripped := strings.TrimSuffix(strings.TrimSuffix(goName, "Names"), "Name")
	switch {
	case stripped == goName:
		return goName + suffix
	case stripped == kind:
		return kind + "Resource" + suffix
	}
	return stripped + kind + suffix
}

func messageName(f *protogen.File, msg *protogen.Message) string {
	return strings.TrimPrefix(string(msg.Desc.FullName()), string(f.Desc.Package())+".")
}

func fieldKind(fld *protogen.Field) string {
	k := fld.Desc.Kind().String()
	if fld.Desc.IsList() {
		return "repeated " + k
	}
	return k
}

// where formats "path:line:col" for a descriptor, falling back to the path
// when the request carries no source info.
func where(f *protogen.File, desc protoreflect.Descriptor) string {
	return position(f, f.Desc.SourceLocations().ByDescriptor(desc))
}

// definitionWhere locates the i-th resource_definition file option.
func definitionWhere(f *protogen.File, i int, typ string) string {
	const fileOptions, resourceDefinition = 8, 1053
	locs := f.Desc.SourceLocations()
	loc := locs.ByPath(protoreflect.SourcePath{fileOptions, resourceDefinition, int32(i)})
	if loc.Path == nil {
		loc = locs.ByPath(protoreflect.SourcePath{fileOptions, resourceDefinition})
	}
	return fmt.Sprintf("%s: resource_definition %q", position(f, loc), typ)
}

func position(f *protogen.File, loc protoreflect.SourceLocation) string {
	if loc.Path == nil {
		return f.Desc.Path()
	}
	return fmt.Sprintf("%s:%d:%d", f.Desc.Path(), loc.StartLine+1, loc.StartColumn+1)
}
