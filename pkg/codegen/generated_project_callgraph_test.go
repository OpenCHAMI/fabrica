// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

type generatedCallGraph map[string]map[string]struct{}

func verifyDedicatedStorageCallers(root string, resources []string) error {
	graph, err := parseGeneratedStorageCallGraph(root)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		plural := resource + "s"
		required := map[string][]string{
			"Save" + resource:            {"ToEnt" + resource, "Update" + resource + "FromResource"},
			"Load" + resource:            {"FromEnt" + resource},
			"LoadAll" + plural:           {"List" + plural, "FromEnt" + resource},
			"Delete" + resource:          {"Delete" + resource + "ByUID"},
			"Get" + resource + "ByUID":   {"FromEnt" + resource},
			"Get" + resource + "ByName":  {"Query" + resource + "ByName", "FromEnt" + resource},
			"List" + plural + "ByLabels": {"List" + plural, "FromEnt" + resource},
			"Query" + plural:             {"entClient." + resource + ".Query"},
		}
		for entrypoint, targets := range required {
			root := generatedFunctionName(graph, entrypoint)
			if root == "" {
				return fmt.Errorf("generated public entrypoint %s is absent", entrypoint)
			}
			for _, target := range targets {
				if !generatedCallReachable(graph, root, target, make(map[string]bool)) {
					return fmt.Errorf("generated entrypoint %s cannot reach dedicated call %s", root, target)
				}
			}
		}
	}
	return nil
}

func generatedFunctionName(graph generatedCallGraph, expected string) string {
	for function := range graph {
		if strings.EqualFold(function, expected) {
			return function
		}
	}
	return ""
}

func parseGeneratedStorageCallGraph(root string) (generatedCallGraph, error) {
	graph := make(generatedCallGraph)
	for _, name := range []string{"storage_ent_resources_generated.go", "ent_queries_generated.go"} {
		path := filepath.Join(root, "internal", "storage", name)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse generated caller %s: %w", path, err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			calls := make(map[string]struct{})
			ast.Inspect(function.Body, func(node ast.Node) bool {
				if _, nested := node.(*ast.FuncLit); nested {
					return false
				}
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if name := generatedCallName(call.Fun); name != "" {
					calls[name] = struct{}{}
				}
				return true
			})
			graph[function.Name.Name] = calls
		}
	}
	return graph, nil
}

func generatedCallName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.SelectorExpr:
		prefix := generatedCallName(expression.X)
		if prefix == "" {
			return expression.Sel.Name
		}
		return prefix + "." + expression.Sel.Name
	default:
		return ""
	}
}

func generatedCallReachable(graph generatedCallGraph, current, target string, visited map[string]bool) bool {
	if visited[current] {
		return false
	}
	visited[current] = true
	for callee := range graph[current] {
		if callee == target || generatedCallReachable(graph, callee, target, visited) {
			return true
		}
	}
	return false
}
