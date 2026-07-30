// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

// StorageBackend identifies the selected generated persistence implementation.
type StorageBackend string

const (
	// StorageBackendEnt identifies generated Ent storage.
	StorageBackendEnt StorageBackend = "ent"
	// StorageBackendFile identifies generated file storage.
	StorageBackendFile StorageBackend = "file"
)

// StorageValidationContext identifies the source and selected persistence contract.
type StorageValidationContext struct {
	Filename     string
	ResourceName string
	Backend      StorageBackend
	Mode         StorageMode
}

// ValidateStorageEnforcement rejects directives the selected persistence contract cannot enforce.
func ValidateStorageEnforcement(resourceAnnotations *ResourceAnnotations, context StorageValidationContext) error {
	if context.Mode == StorageModeDedicated && context.Backend != StorageBackendEnt {
		source, err := resourceDirectiveSource(context, "+fabrica:storage=dedicated")
		if err != nil {
			return err
		}
		return capabilityError(source, CapabilityBackend, fmt.Sprintf("backend %q cannot enforce dedicated storage", context.Backend))
	}
	if context.Mode != StorageModeGeneric || len(resourceAnnotations.Fields) == 0 {
		return nil
	}
	source, found, err := firstFieldDirectiveSource(context, resourceAnnotations)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return capabilityError(source, CapabilityBackend, fmt.Sprintf("backend %q generic storage cannot enforce field directives", context.Backend))
}

func resourceDirectiveSource(context StorageValidationContext, directive string) (SourcePosition, error) {
	fset, declarations, err := validationDeclarations(context)
	if err != nil {
		return SourcePosition{}, err
	}
	declaration, exists := declarations[context.ResourceName]
	if !exists {
		return SourcePosition{Filename: context.Filename, TypeName: context.ResourceName, Directive: directive}, nil
	}
	for _, comment := range declarationComments(declaration) {
		if CleanAnnotationLine(comment.Text) == directive {
			return sourceAt(fset.Position(comment.Slash), SourcePosition{TypeName: context.ResourceName, Directive: directive}), nil
		}
	}
	return sourceAt(fset.Position(declaration.spec.Pos()), SourcePosition{TypeName: context.ResourceName, Directive: directive}), nil
}

func firstFieldDirectiveSource(context StorageValidationContext, resourceAnnotations *ResourceAnnotations) (SourcePosition, bool, error) {
	fset, declarations, err := validationDeclarations(context)
	if err != nil {
		return SourcePosition{}, false, err
	}
	declaration, exists := declarations[context.ResourceName+"Spec"]
	if !exists {
		return SourcePosition{}, false, nil
	}
	structType, ok := declaration.spec.Type.(*ast.StructType)
	if !ok {
		return SourcePosition{}, false, nil
	}
	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			fieldAnnotations := resourceAnnotations.Fields[name.Name]
			if fieldAnnotations == nil || len(fieldAnnotations.RawAnnotations) == 0 {
				continue
			}
			for _, comment := range declarationFieldComments(field) {
				directive := CleanAnnotationLine(comment.Text)
				if IsFabricaAnnotation(directive) {
					return sourceAt(fset.Position(comment.Slash), SourcePosition{TypeName: declaration.spec.Name.Name, FieldName: name.Name, Directive: directive}), true, nil
				}
			}
			return sourceAt(fset.Position(field.Pos()), SourcePosition{TypeName: declaration.spec.Name.Name, FieldName: name.Name, Directive: fieldAnnotations.RawAnnotations[0]}), true, nil
		}
	}
	return SourcePosition{}, false, nil
}

func validationDeclarations(context StorageValidationContext) (*token.FileSet, map[string]typeDeclaration, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, context.Filename, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse annotation validation source %s: %w", context.Filename, err)
	}
	return fset, findTypeDeclarations(file, context.ResourceName), nil
}

func declarationComments(declaration typeDeclaration) []*ast.Comment {
	if declaration.doc == nil {
		return nil
	}
	return declaration.doc.List
}

func declarationFieldComments(field *ast.Field) []*ast.Comment {
	if field.Doc == nil {
		return nil
	}
	return field.Doc.List
}
