// Package rewrite handles inline rewriting of emitted JavaScript files.
// It replaces marker function calls (is, validate, assert, stringify, serialize)
// with direct calls to companion functions, and injects body validation
// into NestJS controller methods.
package rewrite

import (
	"sort"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
)

// MarkerCall represents a detected call to a tsgonest marker function
// (is, validate, assert, stringify, serialize) with a resolved type argument.
type MarkerCall struct {
	FunctionName string // "is", "validate", "assert", "stringify", "serialize"
	TypeName     string // resolved type name e.g. "CreateUserDto"
	SourcePos    int    // character offset in source file (for ordering)
}

// markerFunctions is the set of function names that tsgonest recognizes as markers.
var markerFunctions = map[string]bool{
	"is":        true,
	"validate":  true,
	"assert":    true,
	"stringify": true,
	"serialize": true,
}

// ExtractMarkerCalls finds tsgonest marker calls in a source file.
// It checks imports for `from "tsgonest"`, walks AST for CallExpression
// nodes using those imports with type arguments, and resolves the type
// argument to a named type via the checker.
//
// Returns nil if the file has no tsgonest imports.
func ExtractMarkerCalls(sf *ast.SourceFile, checker *shimchecker.Checker) []MarkerCall {
	// Step 1: Find tsgonest import and collect imported names
	importedNames := findTsgonestImports(sf)
	if len(importedNames) == 0 {
		return nil
	}

	// Step 2: Walk AST to find call expressions using imported marker names
	var calls []MarkerCall
	walkNode(sf.AsNode(), importedNames, checker, &calls)

	// Step 3: Sort by source position for deterministic ordering
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].SourcePos < calls[j].SourcePos
	})

	return calls
}

// findTsgonestImports scans top-level statements for import declarations
// with module specifier "tsgonest" and returns a map of local name → original name.
func findTsgonestImports(sf *ast.SourceFile) map[string]string {
	result := make(map[string]string)

	for _, stmt := range sf.Statements.Nodes {
		if stmt.Kind != ast.KindImportDeclaration {
			continue
		}
		decl := stmt.AsImportDeclaration()

		// Check module specifier is "tsgonest"
		if decl.ModuleSpecifier == nil {
			continue
		}
		if decl.ModuleSpecifier.Kind != ast.KindStringLiteral {
			continue
		}
		moduleSpec := decl.ModuleSpecifier.AsStringLiteral().Text
		if moduleSpec != "tsgonest" {
			continue
		}

		// Extract named imports
		if decl.ImportClause == nil {
			continue
		}
		clause := decl.ImportClause.AsImportClause()
		if clause.NamedBindings == nil {
			continue
		}
		if clause.NamedBindings.Kind != ast.KindNamedImports {
			continue
		}
		namedImports := clause.NamedBindings.AsNamedImports()
		if namedImports.Elements == nil {
			continue
		}
		for _, elem := range namedImports.Elements.Nodes {
			spec := elem.AsImportSpecifier()
			if spec.IsTypeOnly {
				continue
			}
			localName := spec.Name().Text()
			originalName := localName
			if spec.PropertyName != nil {
				originalName = spec.PropertyName.AsIdentifier().Text
			}
			if markerFunctions[originalName] {
				result[localName] = originalName
			}
		}
	}

	return result
}

// walkNode recursively walks the AST looking for CallExpression nodes
// that match marker function calls with type arguments.
func walkNode(node *ast.Node, importedNames map[string]string, checker *shimchecker.Checker, calls *[]MarkerCall) {
	if node == nil {
		return
	}

	if node.Kind == ast.KindCallExpression {
		call := node.AsCallExpression()
		if call.TypeArguments != nil && len(call.TypeArguments.Nodes) == 1 {
			// Check if callee is an identifier matching an imported marker name
			if call.Expression.Kind == ast.KindIdentifier {
				calleeName := call.Expression.AsIdentifier().Text
				if origName, ok := importedNames[calleeName]; ok {
					// Resolve type argument to a named type
					typeNode := call.TypeArguments.Nodes[0]
					typeName := resolveTypeArgName(typeNode, checker)
					if typeName != "" {
						*calls = append(*calls, MarkerCall{
							FunctionName: origName,
							TypeName:     typeName,
							SourcePos:    node.Pos(),
						})
					}
				}
			}
		}
	}

	// Recurse into children
	node.ForEachChild(func(child *ast.Node) bool {
		walkNode(child, importedNames, checker, calls)
		return false // continue visiting
	})
}

// resolveTypeArgName resolves a type argument node to a named type string.
// Uses the checker to get the type, then extracts the symbol name.
func resolveTypeArgName(typeNode *ast.Node, checker *shimchecker.Checker) string {
	resolvedType := shimchecker.Checker_getTypeFromTypeNode(checker, typeNode)
	if resolvedType == nil {
		return ""
	}

	sym := shimchecker.Type_symbol(resolvedType)
	if sym == nil {
		return ""
	}

	return sym.Name
}
