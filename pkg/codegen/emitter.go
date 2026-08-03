// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"sort"
	"sync"
	"text/template"

	"github.com/openchami/fabrica/pkg/annotations"
)

// EmitterKind names a point in generation that a service may customize.
//
// Kinds are stable strings so a project can override generation without
// importing internal identifiers.
type EmitterKind string

const (
	// EmitterKindEntSchema renders the dedicated Ent schema for one resource,
	// i.e. resources annotated +fabrica:storage=dedicated.
	EmitterKindEntSchema EmitterKind = "ent-schema"
)

// EmitRequest carries everything an emitter needs to render one resource.
//
// Fields describe intent and context, not Ent internals, so an emitter can
// target a different backend entirely.
type EmitRequest struct {
	// Kind is the emission point being served.
	Kind EmitterKind

	// Resource is the resource being emitted.
	Resource ResourceMetadata

	// Annotations are the parsed annotations for Resource. Never nil; a
	// resource with no annotations gets the zero value.
	Annotations *annotations.ResourceAnnotations

	// StorageType is the configured backend ("file", "ent").
	StorageType string

	// DBDriver is the configured database driver ("postgres", "mysql", "sqlite").
	DBDriver string

	// ModulePath is the target project's Go module path.
	ModulePath string

	// PackageName is the package being generated into.
	PackageName string

	// DefaultPath is the path the built-in emitter would write, relative to
	// the project root. An emitter may reuse or ignore it.
	DefaultPath string

	// Template is the built-in template for this kind, already parsed. An
	// emitter may execute it, wrap it, or ignore it. Nil if the built-in
	// template is unavailable.
	Template *template.Template

	// Data is the template data the built-in emitter would use. Emitters that
	// execute Template should pass this through so generated output keeps the
	// common fields (version, copyright year, module path).
	Data map[string]interface{}
}

// EmittedFile is one file an emitter wants written.
type EmittedFile struct {
	// Path is relative to the project root. Must be non-empty.
	Path string

	// Content is the file body. Go files are gofmt'd before writing; a file
	// that does not parse is a generation error.
	Content []byte
}

// ResourceEmitter renders artifacts for a single resource.
//
// The built-in Ent schema generator implements this interface, so a custom
// emitter is not a second-class citizen: it runs through exactly the same path.
//
// Implementations must be safe for concurrent use; the generator does not
// serialize calls across resources.
type ResourceEmitter interface {
	// Name identifies the emitter in errors and verbose output.
	Name() string

	// Emit renders artifacts for one resource. Returning an empty slice is
	// legal and means "write nothing for this resource".
	Emit(req EmitRequest) ([]EmittedFile, error)
}

// emitterRegistry holds process-wide emitter overrides.
//
// Generation runs inside a program fabrica writes (cmd/.fabrica-codegen), whose
// source a project cannot edit. That program does import the project's own
// pkg/resources, so an init() there can register an override before main runs.
// This registry is the seam that makes that possible.
type emitterRegistry struct {
	mu       sync.RWMutex
	emitters map[EmitterKind]ResourceEmitter
}

var globalEmitters = &emitterRegistry{
	emitters: make(map[EmitterKind]ResourceEmitter),
}

// RegisterEmitter installs a process-wide emitter for a kind, replacing the
// built-in. Intended to be called from an init() in a package the generated
// codegen runner already imports:
//
//	package resources
//
//	func init() {
//	    codegen.RegisterEmitter(codegen.EmitterKindEntSchema, &myEmitter{})
//	}
//
// Registering a nil emitter, or an unknown kind, is an error. Prefer
// (*Generator).SetEmitter when you own the Generator instance — it is scoped
// and easier to reason about in tests.
func RegisterEmitter(kind EmitterKind, emitter ResourceEmitter) error {
	if emitter == nil {
		return fmt.Errorf("register emitter %q: emitter is nil", kind)
	}
	if !isKnownEmitterKind(kind) {
		return fmt.Errorf("register emitter: unknown kind %q, expected one of %v", kind, KnownEmitterKinds())
	}

	globalEmitters.mu.Lock()
	defer globalEmitters.mu.Unlock()
	globalEmitters.emitters[kind] = emitter

	return nil
}

// UnregisterEmitter removes a process-wide override, restoring the built-in.
func UnregisterEmitter(kind EmitterKind) {
	globalEmitters.mu.Lock()
	defer globalEmitters.mu.Unlock()
	delete(globalEmitters.emitters, kind)
}

// lookupGlobalEmitter returns a registered override, if any.
func lookupGlobalEmitter(kind EmitterKind) (ResourceEmitter, bool) {
	globalEmitters.mu.RLock()
	defer globalEmitters.mu.RUnlock()
	e, ok := globalEmitters.emitters[kind]

	return e, ok
}

// KnownEmitterKinds lists the kinds a service may override, sorted.
func KnownEmitterKinds() []EmitterKind {
	kinds := make([]EmitterKind, 0, len(builtinEmitters))
	for k := range builtinEmitters {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })

	return kinds
}

func isKnownEmitterKind(kind EmitterKind) bool {
	_, ok := builtinEmitters[kind]
	return ok
}

// builtinEmitters maps each kind to fabrica's default implementation.
var builtinEmitters = map[EmitterKind]ResourceEmitter{
	EmitterKindEntSchema: entSchemaEmitter{},
}

// BuiltinEmitter returns fabrica's default emitter for a kind, or nil if the
// kind is unknown.
//
// This is what makes partial customization practical: an emitter can handle the
// resources it cares about and delegate the rest, instead of reimplementing
// fabrica's output.
//
//	func (e myEmitter) Emit(req codegen.EmitRequest) ([]codegen.EmittedFile, error) {
//	    if !e.handles(req.Resource) {
//	        return codegen.BuiltinEmitter(req.Kind).Emit(req)
//	    }
//	    ...
//	}
func BuiltinEmitter(kind EmitterKind) ResourceEmitter {
	return builtinEmitters[kind]
}

// SetEmitter overrides the emitter for a kind on this Generator only.
//
// This takes precedence over RegisterEmitter and is the right choice in tests
// and anywhere the caller owns the Generator.
func (g *Generator) SetEmitter(kind EmitterKind, emitter ResourceEmitter) error {
	if emitter == nil {
		return fmt.Errorf("set emitter %q: emitter is nil", kind)
	}
	if !isKnownEmitterKind(kind) {
		return fmt.Errorf("set emitter: unknown kind %q, expected one of %v", kind, KnownEmitterKinds())
	}

	if g.Emitters == nil {
		g.Emitters = make(map[EmitterKind]ResourceEmitter)
	}
	g.Emitters[kind] = emitter

	return nil
}

// emitterFor resolves the emitter for a kind, most specific first:
// per-Generator override, then process-wide registration, then the built-in.
func (g *Generator) emitterFor(kind EmitterKind) ResourceEmitter {
	if e, ok := g.Emitters[kind]; ok && e != nil {
		return e
	}
	if e, ok := lookupGlobalEmitter(kind); ok && e != nil {
		return e
	}

	return builtinEmitters[kind]
}

// entSchemaEmitter is fabrica's built-in dedicated Ent schema generator.
//
// It renders the embedded ent/schema/resource_dedicated.go.tmpl. Keeping it
// behind the same interface a service would implement means the extension point
// is exercised on every single generation, not only when someone overrides it.
type entSchemaEmitter struct{}

func (entSchemaEmitter) Name() string { return "builtin/ent-schema" }

func (entSchemaEmitter) Emit(req EmitRequest) ([]EmittedFile, error) {
	if req.Template == nil {
		return nil, fmt.Errorf("ent schema template not loaded")
	}

	content, err := renderTemplate(req.Template, req.Data)
	if err != nil {
		return nil, err
	}

	return []EmittedFile{{Path: req.DefaultPath, Content: content}}, nil
}
