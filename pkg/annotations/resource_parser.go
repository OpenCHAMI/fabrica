// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"go/ast"
	"go/token"
)

// ParseResourceAnnotations extracts Fabrica annotations from a type declaration.
func ParseResourceAnnotations(typeSpec *ast.TypeSpec, docComments *ast.CommentGroup) (*ResourceAnnotations, error) {
	return parseResourceAnnotations(typeSpec, docComments, nil)
}

func parseResourceAnnotations(
	typeSpec *ast.TypeSpec,
	docComments *ast.CommentGroup,
	fset *token.FileSet,
) (*ResourceAnnotations, error) {
	result := NewResourceAnnotations()
	seenResourceDirectives := make(map[string]string)
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
			if err := parseResourceLevelAnnotation(result, line, seenResourceDirectives); err != nil {
				return nil, contextualizeParseError(err, parseSource{
					position:  sourcePosition(fset, comment.Slash),
					typeName:  typeSpec.Name.Name,
					directive: line,
				})
			}
		}
	}

	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return result, nil
	}
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 || field.Doc == nil {
			continue
		}
		fieldName := field.Names[0].Name
		fieldAnnotations := NewFieldAnnotations(fieldName)
		seenFieldDirectives := make(map[string]string)
		for _, comment := range field.Doc.List {
			line := CleanAnnotationLine(comment.Text)
			if !IsFabricaAnnotation(line) {
				continue
			}
			fieldAnnotations.RawAnnotations = append(fieldAnnotations.RawAnnotations, line)
			if err := parseFieldLevelAnnotation(fieldAnnotations, line, seenFieldDirectives); err != nil {
				return nil, contextualizeParseError(err, parseSource{
					position:  sourcePosition(fset, comment.Slash),
					typeName:  typeSpec.Name.Name,
					fieldName: fieldName,
					directive: line,
				})
			}
		}
		if len(fieldAnnotations.RawAnnotations) > 0 {
			result.Fields[fieldName] = fieldAnnotations
		}
	}
	return result, nil
}

func sourcePosition(fset *token.FileSet, position token.Pos) token.Position {
	if fset == nil {
		return token.Position{}
	}
	return fset.Position(position)
}
