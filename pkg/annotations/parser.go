// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// ParseResourceAnnotations extracts Fabrica annotations from a type declaration
//
// This parses annotations from the type's doc comments and field comments.
// The docComments parameter should be the GenDecl.Doc if available, or typeSpec.Doc otherwise.
//
// Example:
//
//	genDecl := ... // *ast.GenDecl from parsing Go code
//	typeSpec := genDecl.Specs[0].(*ast.TypeSpec)
//	annotations, err := ParseResourceAnnotations(typeSpec, genDecl.Doc)
func ParseResourceAnnotations(typeSpec *ast.TypeSpec, docComments *ast.CommentGroup) (*ResourceAnnotations, error) {
	return parseResourceAnnotations(typeSpec, docComments, nil)
}

type packageContext struct {
	aliases  map[string]FieldTypeInfo
	typeInfo map[ast.Expr]FieldTypeInfo
	files    []string    // absolute paths of package Go files
	astFiles []*ast.File // corresponding parsed AST files
}

func parseResourceAnnotations(typeSpec *ast.TypeSpec, docComments *ast.CommentGroup, ctx *packageContext) (*ResourceAnnotations, error) {
	if typeSpec == nil {
		return nil, &ParseError{Message: "typeSpec must not be nil"}
	}

	result := NewResourceAnnotations()

	// Parse type-level annotations from doc comments
	// Try the provided docComments first (from GenDecl), then fall back to typeSpec.Doc
	comments := docComments
	if comments == nil {
		comments = typeSpec.Doc
	}

	if comments != nil {
		for _, comment := range comments.List {
			line := CleanAnnotationLine(comment.Text)
			if !IsFabricaAnnotation(line) {
				continue
			}

			result.RawAnnotations = append(result.RawAnnotations, line)

			if err := parseResourceLevelAnnotation(result, line); err != nil {
				return nil, err
			}
		}
	}

	// Parse field-level annotations if this is a struct
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return result, nil
	}

	for _, field := range structType.Fields.List {
		// Skip fields without names (embedded types)
		if len(field.Names) == 0 {
			continue
		}

		for _, name := range field.Names {
			result.SpecFields[name.Name] = true
		}

		// Parse annotations from field comments
		if field.Doc != nil {
			fieldAnnotations := NewFieldAnnotations(field.Names[0].Name)
			fieldAnnotations.FieldType = formatExpr(field.Type)
			fieldAnnotations.TypeInfo = fieldTypeInfoFromContext(field.Type, ctx)

			for _, comment := range field.Doc.List {
				line := CleanAnnotationLine(comment.Text)
				if !IsFabricaAnnotation(line) {
					continue
				}

				fieldAnnotations.RawAnnotations = append(fieldAnnotations.RawAnnotations, line)

				if err := parseFieldLevelAnnotation(fieldAnnotations, line); err != nil {
					return nil, &ParseError{Line: line, Message: err.Error()}
				}
			}

			// Only store if field has annotations
			if len(fieldAnnotations.RawAnnotations) > 0 {
				for _, name := range field.Names {
					result.Fields[name.Name] = cloneFieldAnnotations(fieldAnnotations, name.Name)
				}
			}
		}
	}

	return result, nil
}

// parseResourceLevelAnnotation processes a single resource-level annotation
func parseResourceLevelAnnotation(result *ResourceAnnotations, annotation string) error {
	parts := ParseAnnotationValue(annotation)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "resource":
		if len(parts) != 1 {
			return &ParseError{Line: annotation, Message: "resource annotation does not accept parameters"}
		}
		result.IsResource = true
		return nil

	case "index":
		return parseCompositeIndexAnnotation(result, parts[1:], annotation)

	default:
		// Try to parse as key=value format (e.g., storage=dedicated)
		key, value, hasValue := ParseKeyValue(parts[0])
		if !hasValue {
			return &ParseError{Line: annotation, Message: fmt.Sprintf("unknown resource annotation %q", parts[0])}
		}

		if key == "migration" {
			if len(parts) != 1 {
				return &ParseError{Line: annotation, Message: "migration annotation does not accept trailing parameters"}
			}
			switch MigrationPolicy(value) {
			case MigrationPolicyUnrestricted, MigrationPolicyAdditiveOnly:
				result.Migration = MigrationPolicy(value)
			default:
				return &ParseError{
					Line:    annotation,
					Message: fmt.Sprintf("unknown migration policy %q, expected 'unrestricted' or 'additive-only'", value),
				}
			}
			return nil
		}

		if key == "storage" {
			if len(parts) != 1 {
				return &ParseError{Line: annotation, Message: "storage annotation does not accept trailing parameters"}
			}
			switch StorageMode(value) {
			case StorageModeGeneric:
				result.StorageMode = StorageModeGeneric
			case StorageModeDedicated:
				result.StorageMode = StorageModeDedicated
			default:
				return &ParseError{
					Line:    annotation,
					Message: fmt.Sprintf("unknown storage mode %q, expected 'generic' or 'dedicated'", value),
				}
			}
			return nil
		}

		return &ParseError{Line: annotation, Message: fmt.Sprintf("unknown resource annotation %q", key)}
	}
}

func cloneFieldAnnotations(source *FieldAnnotations, fieldName string) *FieldAnnotations {
	clone := *source
	clone.FieldName = fieldName
	clone.RawAnnotations = append([]string(nil), source.RawAnnotations...)
	return &clone
}

func formatExpr(expr ast.Expr) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return buf.String()
}

func collectTypeAliases(file *ast.File) map[string]FieldTypeInfo {
	return collectTypeAliasesFromFiles([]*ast.File{file})
}

func collectTypeAliasesFromFiles(files []*ast.File) map[string]FieldTypeInfo {
	typeExprs := make(map[string]ast.Expr)
	aliases := make(map[string]FieldTypeInfo)
	for _, file := range files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				typeExprs[typeSpec.Name.Name] = typeSpec.Type
				aliases[typeSpec.Name.Name] = fieldTypeInfo(typeSpec.Type, aliases)
			}
		}
	}

	for range typeExprs {
		for name, expr := range typeExprs {
			aliases[name] = fieldTypeInfo(expr, aliases)
		}
	}
	return aliases
}

func fieldTypeInfoFromContext(expr ast.Expr, ctx *packageContext) FieldTypeInfo {
	if ctx != nil {
		if info, ok := ctx.typeInfo[expr]; ok {
			return info
		}
		return fieldTypeInfo(expr, ctx.aliases)
	}
	return fieldTypeInfo(expr, nil)
}

func fieldTypeInfo(expr ast.Expr, aliases map[string]FieldTypeInfo) FieldTypeInfo {
	info := FieldTypeInfo{Syntax: formatExpr(expr), UnderlyingKind: FieldKindUnknown}
	return resolveFieldTypeInfo(expr, aliases, info)
}

func resolveFieldTypeInfo(expr ast.Expr, aliases map[string]FieldTypeInfo, info FieldTypeInfo) FieldTypeInfo {
	switch t := expr.(type) {
	case *ast.Ident:
		if alias, ok := aliases[t.Name]; ok {
			alias.Syntax = info.Syntax
			alias.PointerDepth += info.PointerDepth
			alias.NamedType = t.Name
			return alias
		}
		return builtinFieldTypeInfo(t.Name, info)

	case *ast.StarExpr:
		info.PointerDepth++
		return resolveFieldTypeInfo(t.X, aliases, info)

	case *ast.ArrayType:
		info.UnderlyingKind = FieldKindSlice
		info.IsResolved = true
		info.IsComparable = false
		return info

	case *ast.MapType:
		info.UnderlyingKind = FieldKindMap
		info.IsResolved = true
		info.IsComparable = false
		return info

	case *ast.StructType:
		info.UnderlyingKind = FieldKindStruct
		info.IsResolved = true
		return info

	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok && x.Name == "time" && t.Sel.Name == "Time" {
			info.UnderlyingKind = FieldKindStruct
			info.IsResolved = true
			info.IsTime = true
			info.IsComparable = true
		}
		return info

	default:
		return info
	}
}

func builtinFieldTypeInfo(name string, info FieldTypeInfo) FieldTypeInfo {
	switch name {
	case "string":
		info.UnderlyingKind = FieldKindString
		info.IsResolved = true
		info.IsStringLike = true
		info.IsScalar = true
		info.IsComparable = true
	case "bool":
		info.UnderlyingKind = FieldKindBool
		info.IsResolved = true
		info.IsScalar = true
		info.IsComparable = true
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		info.UnderlyingKind = FieldKindInt
		info.IsResolved = true
		info.IsScalar = true
		info.IsComparable = true
	case "float32", "float64":
		info.UnderlyingKind = FieldKindFloat
		info.IsResolved = true
		info.IsScalar = true
		info.IsComparable = true
	default:
		info.UnderlyingKind = FieldKindUnknown
	}
	return info
}

func parsePackageContext(filename string) (*ast.File, *packageContext, error) {
	fset := token.NewFileSet()
	target, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse file %s: %w", filename, err)
	}

	absFilename, err := filepath.Abs(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve file %s: %w", filename, err)
	}
	files := []*ast.File{target}
	filenames := []string{absFilename}
	dir := filepath.Dir(absFilename)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("read package directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if sameFile(path, absFilename) {
			continue
		}
		match, err := build.Default.MatchFile(dir, entry.Name())
		if err != nil {
			return nil, nil, fmt.Errorf("check build constraints for %s: %w", path, err)
		}
		if !match {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, nil, fmt.Errorf("parse package file %s: %w", path, err)
		}
		if file.Name.Name == target.Name.Name {
			files = append(files, file)
			filenames = append(filenames, path)
		}
	}

	ctx := buildPackageContext(fset, target.Name.Name, files)
	ctx.files = filenames
	ctx.astFiles = files
	return target, ctx, nil
}

func sameFile(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		return absA == absB
	}
	return a == b
}

func buildPackageContext(fset *token.FileSet, pkgName string, files []*ast.File) *packageContext {
	ctx := &packageContext{
		aliases:  collectTypeAliasesFromFiles(files),
		typeInfo: make(map[ast.Expr]FieldTypeInfo),
	}

	typesInfo := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	conf := types.Config{
		Importer: importer.Default(),
		Error:    func(error) {},
	}

	// Ignore the Check result because we only need its best-effort population of typesInfo; unresolved/invalid types are marked unknown and fail closed in validation.
	_, _ = conf.Check(pkgName, fset, files, typesInfo)
	for expr, typeAndValue := range typesInfo.Types {
		if typeAndValue.Type != nil {
			ctx.typeInfo[expr] = fieldTypeInfoFromType(expr, typeAndValue.Type)
		}
	}

	return ctx
}

func fieldTypeInfoFromType(expr ast.Expr, typ types.Type) FieldTypeInfo {
	info := FieldTypeInfo{Syntax: formatExpr(expr), UnderlyingKind: FieldKindUnknown, IsResolved: true}
	if basic, ok := typ.Underlying().(*types.Basic); ok && basic.Kind() == types.Invalid {
		info.IsResolved = false
		return info
	}
	return resolveTypeInfo(typ, info)
}

func resolveTypeInfo(typ types.Type, info FieldTypeInfo) FieldTypeInfo {
	for {
		ptr, ok := typ.(*types.Pointer)
		if !ok {
			break
		}
		info.PointerDepth++
		typ = ptr.Elem()
	}

	if named, ok := typ.(*types.Named); ok {
		info.NamedType = named.Obj().Name()
		if named.Obj().Pkg() != nil {
			if named.Obj().Pkg().Path() == "time" && named.Obj().Name() == "Time" {
				info.UnderlyingKind = FieldKindStruct
				info.IsTime = true
				info.IsComparable = true
				return info
			}
		}
	}

	switch underlying := typ.Underlying().(type) {
	case *types.Basic:
		return basicTypeInfo(underlying, info)
	case *types.Slice, *types.Array:
		info.UnderlyingKind = FieldKindSlice
	case *types.Map:
		info.UnderlyingKind = FieldKindMap
	case *types.Struct:
		info.UnderlyingKind = FieldKindStruct
		info.IsComparable = types.Comparable(typ)
	}
	return info
}

func basicTypeInfo(basic *types.Basic, info FieldTypeInfo) FieldTypeInfo {
	switch basic.Kind() {
	case types.String:
		info.UnderlyingKind = FieldKindString
		info.IsStringLike = true
		info.IsScalar = true
		info.IsComparable = true
	case types.Bool:
		info.UnderlyingKind = FieldKindBool
		info.IsScalar = true
		info.IsComparable = true
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64, types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr:
		info.UnderlyingKind = FieldKindInt
		info.IsScalar = true
		info.IsComparable = true
	case types.Float32, types.Float64:
		info.UnderlyingKind = FieldKindFloat
		info.IsScalar = true
		info.IsComparable = true
	}
	return info
}

// parseFieldLevelAnnotation processes a single field-level annotation
func parseFieldLevelAnnotation(result *FieldAnnotations, annotation string) error {
	parts := ParseAnnotationValue(annotation)
	if len(parts) == 0 {
		return nil
	}

	// All field annotations start with "field:"
	if parts[0] != "field" {
		return &ParseError{
			Line:    annotation,
			Message: "field annotations must start with +fabrica:field:",
		}
	}

	if len(parts) < 2 {
		return &ParseError{
			Line:    annotation,
			Message: "field annotation requires a directive after 'field:'",
		}
	}

	directive := parts[1]

	switch {
	case directive == "sensitive":
		if len(parts) != 2 {
			return fmt.Errorf("sensitive annotation does not accept parameters")
		}
		result.Sensitive = true
		return nil

	case directive == "immutable":
		if len(parts) != 2 {
			return fmt.Errorf("immutable annotation does not accept parameters")
		}
		result.Immutable = true
		return nil

	case directive == "unique":
		if len(parts) != 2 {
			return fmt.Errorf("unique annotation does not accept parameters")
		}
		result.Unique = true
		return nil

	case directive == "nullable":
		if len(parts) != 2 {
			return fmt.Errorf("nullable annotation does not accept parameters")
		}
		result.Nullable = true
		return nil

	case directive == "notnull":
		if len(parts) != 2 {
			return fmt.Errorf("notnull annotation does not accept parameters")
		}
		result.NotNull = true
		return nil

	case strings.HasPrefix(directive, "size"):
		return parseSizeAnnotation(result, parts[1:], annotation)

	case strings.HasPrefix(directive, "relation"):
		return parseRelationAnnotation(result, parts[1:], annotation)

	case strings.HasPrefix(directive, "storage"):
		return parseStorageAnnotation(result, parts[1:], annotation)

	case strings.HasPrefix(directive, "index"):
		return parseIndexAnnotation(result, parts[1:], annotation)

	case strings.HasPrefix(directive, "default"):
		return parseDefaultAnnotation(result, parts[1:], annotation)

	default:
		return fmt.Errorf("unknown field annotation %q", directive)
	}
}

// parseStorageAnnotation parses +fabrica:field:storage=<type>:<details>
//
// Examples:
//   - +fabrica:field:storage=hashed:bcrypt:cost=12
//   - +fabrica:field:storage=encrypted:aes256:key=env
func parseStorageAnnotation(result *FieldAnnotations, parts []string, fullAnnotation string) error {
	if len(parts) == 0 {
		return fmt.Errorf("storage annotation requires parameters")
	}

	// Parse: storage=<type>
	key, value, hasValue := ParseKeyValue(parts[0])
	if !hasValue || key != "storage" {
		return fmt.Errorf("expected format: storage=<type>")
	}

	result.Storage = &StorageConfig{
		Type: StorageType(value),
	}

	switch result.Storage.Type {
	case StorageTypeHashed:
		return parseHashedStorage(result.Storage, parts[1:], fullAnnotation)
	case StorageTypeEncrypted:
		return parseEncryptedStorage(result.Storage, parts[1:], fullAnnotation)
	case StorageTypeDefault:
		return nil
	default:
		return fmt.Errorf("unknown storage type %q, expected 'hashed', 'encrypted', or 'default'", value)
	}
}

// parseHashedStorage parses hashed storage parameters
//
// Format: +fabrica:field:storage=hashed:bcrypt:cost=12
func parseHashedStorage(config *StorageConfig, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("hashed storage requires algorithm: storage=hashed:<algorithm>")
	}

	config.Hash = &HashConfig{
		Algorithm: HashAlgorithm(parts[0]),
	}

	// Parse algorithm-specific parameters
	switch config.Hash.Algorithm {
	case HashAlgorithmBcrypt:
		// Default bcrypt cost
		config.Hash.Cost = 12

		// Parse cost parameter if provided
		if len(parts) > 1 {
			seenCost := false
			for _, param := range parts[1:] {
				key, value, hasValue := ParseKeyValue(param)
				if !hasValue || key != "cost" {
					return fmt.Errorf("unknown bcrypt parameter %q, expected 'cost=<n>'", param)
				}

				if seenCost {
					return fmt.Errorf("duplicate bcrypt cost parameter")
				}
				seenCost = true
				cost, err := ParseIntValue(value, 4, 31)
				if err != nil {
					return fmt.Errorf("bcrypt cost: %w", err)
				}
				config.Hash.Cost = cost
			}
		}
		return nil

	case HashAlgorithmArgon2:
		// Default argon2 parameters
		config.Hash.Cost = 65536 // 64MB memory

		if len(parts) > 1 {
			seenMemory := false
			for _, param := range parts[1:] {
				key, value, hasValue := ParseKeyValue(param)
				if !hasValue || key != "memory" {
					return fmt.Errorf("unknown argon2 parameter %q, expected 'memory=<kb>'", param)
				}

				if seenMemory {
					return fmt.Errorf("duplicate argon2 memory parameter")
				}
				seenMemory = true
				memory, err := ParseIntValue(value, 1024, 1048576)
				if err != nil {
					return fmt.Errorf("argon2 memory: %w", err)
				}
				config.Hash.Cost = memory
			}
		}
		return nil

	case HashAlgorithmSHA256:
		if len(parts) > 1 {
			return fmt.Errorf("sha256 does not accept parameters")
		}
		return nil

	default:
		return fmt.Errorf("unknown hash algorithm %q, expected 'bcrypt', 'argon2', or 'sha256'", config.Hash.Algorithm)
	}
}

// parseEncryptedStorage parses encrypted storage parameters
//
// Format: +fabrica:field:storage=encrypted:aes256:key=env
func parseEncryptedStorage(config *StorageConfig, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("encrypted storage requires algorithm: storage=encrypted:<algorithm>")
	}

	config.Encryption = &EncryptionConfig{
		Algorithm: parts[0],
		KeySource: "env", // default
	}

	// Parse key source if provided
	if len(parts) > 1 {
		seenKey := false
		for _, param := range parts[1:] {
			key, value, hasValue := ParseKeyValue(param)
			if !hasValue || key != "key" {
				return fmt.Errorf("unknown encryption parameter %q, expected 'key=<source>'", param)
			}

			if seenKey {
				return fmt.Errorf("duplicate encryption key parameter")
			}
			seenKey = true
			config.Encryption.KeySource = value
		}
	}

	return nil
}

// parseIndexAnnotation parses +fabrica:field:index or +fabrica:field:index=<type>
//
// Examples:
//   - +fabrica:field:index
//   - +fabrica:field:index=gin
func parseIndexAnnotation(result *FieldAnnotations, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("index annotation missing")
	}

	result.Index = &IndexConfig{
		Type: IndexTypeBTree, // default
	}

	key, value, hasValue := ParseKeyValue(parts[0])
	if key != "index" {
		return fmt.Errorf("expected 'index' key")
	}

	if hasValue {
		switch IndexType(value) {
		case IndexTypeBTree, IndexTypeGIN, IndexTypeGiST, IndexTypeHash:
			result.Index.Type = IndexType(value)
		default:
			return fmt.Errorf("unknown index type %q, expected 'btree', 'gin', 'gist', or 'hash'", value)
		}
	}

	// Trailing modifiers: unique and name=<identifier>
	seenUnique := false
	seenName := false
	for _, param := range parts[1:] {
		pKey, pValue, pHasValue := ParseKeyValue(param)
		switch {
		case pKey == "unique" && !pHasValue:
			if seenUnique {
				return fmt.Errorf("duplicate index unique modifier")
			}
			seenUnique = true
			result.Index.Unique = true
		case pKey == "name" && pHasValue:
			if seenName {
				return fmt.Errorf("duplicate index name modifier")
			}
			if !isPortableIdentifier(pValue) {
				return fmt.Errorf("invalid index name %q", pValue)
			}
			seenName = true
			result.Index.Name = pValue
		default:
			return fmt.Errorf("unknown index modifier %q, expected 'unique' or 'name=<identifier>'", param)
		}
	}

	return nil
}

// parseCompositeIndexAnnotation parses a resource-level multi-column index.
//
// Format: +fabrica:index:fields=<f1,f2>[:name=<identifier>][:unique][:type=<type>]
func parseCompositeIndexAnnotation(result *ResourceAnnotations, parts []string, annotation string) error {
	idx := &CompositeIndex{Type: IndexTypeBTree}
	seenFields := false
	seenName := false
	seenUnique := false
	seenType := false

	for _, param := range parts {
		key, value, hasValue := ParseKeyValue(param)
		switch {
		case key == "fields" && hasValue:
			if seenFields {
				return &ParseError{Line: annotation, Message: "duplicate composite index fields parameter"}
			}
			seenFields = true
			for f := range strings.SplitSeq(value, ",") {
				if f = strings.TrimSpace(f); f != "" {
					if !isExportedGoIdentifier(f) {
						return &ParseError{
							Line:    annotation,
							Message: fmt.Sprintf("composite index field %q is not a valid Go field name", f),
						}
					}
					idx.Fields = append(idx.Fields, f)
				}
			}

		case key == "name" && hasValue:
			if seenName {
				return &ParseError{Line: annotation, Message: "duplicate composite index name parameter"}
			}
			if !isPortableIdentifier(value) {
				return &ParseError{Line: annotation, Message: fmt.Sprintf("invalid index name %q", value)}
			}
			seenName = true
			idx.Name = value

		case key == "unique" && !hasValue:
			if seenUnique {
				return &ParseError{Line: annotation, Message: "duplicate composite index unique parameter"}
			}
			seenUnique = true
			idx.Unique = true

		case key == "type" && hasValue:
			if seenType {
				return &ParseError{Line: annotation, Message: "duplicate composite index type parameter"}
			}
			seenType = true
			switch IndexType(value) {
			case IndexTypeBTree, IndexTypeGIN, IndexTypeGiST, IndexTypeHash:
				idx.Type = IndexType(value)
			default:
				return &ParseError{
					Line:    annotation,
					Message: fmt.Sprintf("unknown index type %q, expected 'btree', 'gin', 'gist', or 'hash'", value),
				}
			}

		default:
			return &ParseError{
				Line:    annotation,
				Message: fmt.Sprintf("unknown composite index parameter %q, expected 'fields=', 'name=', 'unique', or 'type='", param),
			}
		}
	}

	if len(idx.Fields) == 0 {
		return &ParseError{
			Line:    annotation,
			Message: "composite index requires fields=<f1,f2>",
		}
	}

	result.Indexes = append(result.Indexes, idx)
	return nil
}

// parseSizeAnnotation parses +fabrica:field:size=<n>
func parseSizeAnnotation(result *FieldAnnotations, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("size annotation missing")
	}

	key, value, hasValue := ParseKeyValue(parts[0])
	if !hasValue || key != "size" {
		return fmt.Errorf("expected format: size=<n>")
	}
	if len(parts) != 1 {
		return fmt.Errorf("size annotation does not accept trailing parameters")
	}

	size, err := ParseIntValue(value, 1, 65535)
	if err != nil {
		return fmt.Errorf("field size: %w", err)
	}

	result.Size = size
	return nil
}

// parseRelationAnnotation parses a foreign-key relation to another resource.
//
// Format: +fabrica:field:relation=<belongs-to|has-many>:<Target>[:on-delete=<action>]
func parseRelationAnnotation(result *FieldAnnotations, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("relation annotation missing")
	}

	key, value, hasValue := ParseKeyValue(parts[0])
	if !hasValue || key != "relation" {
		return fmt.Errorf("expected format: relation=<belongs-to|has-many>:<Target>")
	}

	kind := RelationKind(value)
	switch kind {
	case RelationBelongsTo, RelationHasMany:
	default:
		return fmt.Errorf("unknown relation kind %q, expected 'belongs-to' or 'has-many'", value)
	}

	if len(parts) < 2 {
		return fmt.Errorf("relation requires a target resource type: relation=%s:<Target>", value)
	}

	rel := &RelationConfig{
		Kind:     kind,
		Target:   parts[1],
		OnDelete: OnDeleteRestrict,
	}

	for _, param := range parts[2:] {
		pKey, pValue, pHasValue := ParseKeyValue(param)
		if pKey != "on-delete" || !pHasValue {
			return fmt.Errorf("unknown relation parameter %q, expected 'on-delete=<action>'", param)
		}

		switch OnDeleteAction(pValue) {
		case OnDeleteRestrict, OnDeleteCascade, OnDeleteSetNull:
			rel.OnDelete = OnDeleteAction(pValue)
		default:
			return fmt.Errorf("unknown on-delete action %q, expected 'restrict', 'cascade', or 'set-null'", pValue)
		}
	}

	result.Relation = rel
	return nil
}

// parseDefaultAnnotation parses +fabrica:field:default=<value>
//
// Example:
//   - +fabrica:field:default=true
//   - +fabrica:field:default=0
func parseDefaultAnnotation(result *FieldAnnotations, parts []string, _ string) error {
	if len(parts) == 0 {
		return fmt.Errorf("default annotation missing")
	}

	key, value, hasValue := ParseKeyValue(parts[0])
	if !hasValue || key != "default" {
		return fmt.Errorf("expected format: default=<value>")
	}
	if len(parts) != 1 {
		return fmt.Errorf("default annotation does not accept trailing parameters")
	}
	if value == "" {
		return fmt.Errorf("default annotation requires a non-empty value")
	}

	result.Default = value
	return nil
}

// ParseResourceFile returns the annotations for a single resource type in a
// file, merging the field annotations declared on its <Name>Spec companion.
//
// Fabrica splits a resource across two declarations: type-level annotations sit
// on <Name>, field-level annotations on <Name>Spec. Anything that wants a
// complete, validatable picture of a resource needs both, so this does the
// merge in one place.
//
// Declaration order does not matter: <Name>Spec may appear before or after
// <Name>. A resource that does not appear in the file yields empty annotations
// rather than an error, matching the previous behaviour of callers.
//
// The companion <Name>Spec may live in a different file within the same package;
// all package files are searched to locate both declarations.
func ParseResourceFile(filename, resourceName string) (*ResourceAnnotations, error) {
	_, ctx, err := parsePackageContext(filename)
	if err != nil {
		return nil, err
	}
	return parseResourceFromFilesWithContext(ctx.astFiles, resourceName, ctx)
}

// parseResourceFromFile does the two-declaration merge over an already-parsed
// file. Kept separate so callers that already hold an *ast.File do not reparse.
func parseResourceFromFile(file *ast.File, resourceName string) (*ResourceAnnotations, error) {
	return parseResourceFromFileWithContext(file, resourceName, &packageContext{aliases: collectTypeAliases(file)})
}

// parseResourceFromFilesWithContext walks all provided files to find the named
// resource and its companion <Name>Spec, then merges field annotations from both.
func parseResourceFromFilesWithContext(files []*ast.File, resourceName string, ctx *packageContext) (*ResourceAnnotations, error) {
	specTypeName := resourceName + "Spec"

	var (
		resourceAnnots *ResourceAnnotations
		specFields     map[string]*FieldAnnotations
		specFieldNames map[string]bool
	)

	for _, file := range files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				switch typeSpec.Name.Name {
				case resourceName:
					annots, err := parseResourceAnnotations(typeSpec, genDecl.Doc, ctx)
					if err != nil {
						return nil, fmt.Errorf("parse annotations on %s: %w", resourceName, err)
					}
					resourceAnnots = annots

				case specTypeName:
					annots, err := parseResourceAnnotations(typeSpec, genDecl.Doc, ctx)
					if err != nil {
						return nil, fmt.Errorf("parse annotations on %s: %w", specTypeName, err)
					}
					if specFields == nil {
						specFields = make(map[string]*FieldAnnotations)
					}
					if specFieldNames == nil {
						specFieldNames = make(map[string]bool)
					}
					maps.Copy(specFields, annots.Fields)
					maps.Copy(specFieldNames, annots.SpecFields)
				}
			}
		}
	}

	// Merge after all files are walked, so <Name>Spec declared before
	// <Name> is not discarded when the resource declaration is reached.
	if resourceAnnots == nil {
		if specFields == nil {
			return NewResourceAnnotations(), nil
		}
		resourceAnnots = NewResourceAnnotations()
	}

	maps.Copy(resourceAnnots.Fields, specFields)
	maps.Copy(resourceAnnots.SpecFields, specFieldNames)

	return resourceAnnots, nil
}

// parseResourceFromFileWithContext is the single-file variant for backward
// compatibility with callers that already hold a single *ast.File.
func parseResourceFromFileWithContext(file *ast.File, resourceName string, ctx *packageContext) (*ResourceAnnotations, error) {
	return parseResourceFromFilesWithContext([]*ast.File{file}, resourceName, ctx)
}

// mergeSpecFields folds each <Name>Spec's field annotations into <Name> so that
// every resource entry in the map is complete and validatable on its own. The
// <Name>Spec entries are left in place for backward compatibility.
func mergeSpecFields(byType map[string]*ResourceAnnotations) {
	for typeName, annots := range byType {
		spec, ok := byType[typeName+"Spec"]
		if !ok {
			continue
		}
		maps.Copy(annots.Fields, spec.Fields)
		maps.Copy(annots.SpecFields, spec.SpecFields)
	}
}

// ParseFileAnnotations parses annotations from a Go source file with caching
//
// This function parses a Go source file and extracts Fabrica annotations.
// Results are cached based on file modification time for performance.
//
// Example:
//
//	annotations, err := ParseFileAnnotations("apis/v1/user_types.go")
//	if err != nil {
//	    return err
//	}
//	for fieldName, ann := range annotations {
//	    fmt.Printf("%s: %+v\n", fieldName, ann)
//	}
func ParseFileAnnotations(filename string) (map[string]*ResourceAnnotations, error) {
	if cached, ok := globalCache.Get(filename); ok {
		return cached, nil
	}

	file, ctx, err := parsePackageContext(filename)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*ResourceAnnotations)

	// Walk declarations
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			annotations, err := parseResourceAnnotations(typeSpec, genDecl.Doc, ctx)
			if err != nil {
				return nil, err
			}

			result[typeSpec.Name.Name] = annotations
		}
	}

	mergeSpecFields(result)
	globalCache.SetWithDependencies(filename, result, ctx.files)

	return result, nil
}
