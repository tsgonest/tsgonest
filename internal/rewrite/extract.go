// Package rewrite handles inline rewriting of emitted JavaScript files.
// It replaces marker function calls (is, validate, assert, stringify, serialize)
// with direct calls to companion functions, and injects body validation
// into NestJS controller methods.
package rewrite

import (
	"fmt"
	"sort"
	"strings"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	shimscanner "github.com/microsoft/typescript-go/shim/scanner"
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
// Returns nil calls if the file has no tsgonest imports. Warnings name marker
// calls whose type argument could not be resolved to a named type — those
// calls stay unrewritten (runtime no-op) so users must not be left guessing.
func ExtractMarkerCalls(sf *ast.SourceFile, checker *shimchecker.Checker) ([]MarkerCall, []string) {
	// Step 1: Find tsgonest import and collect imported names
	importedNames := findTsgonestImports(sf)
	if len(importedNames) == 0 {
		return nil, nil
	}

	// Step 2: Walk AST to find call expressions using imported marker names
	var calls []MarkerCall
	var warnings []string
	walkNode(sf, sf.AsNode(), importedNames, checker, &calls, &warnings)

	// Step 3: Sort by source position for deterministic ordering
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].SourcePos < calls[j].SourcePos
	})

	return calls, warnings
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
func walkNode(sf *ast.SourceFile, node *ast.Node, importedNames map[string]string, checker *shimchecker.Checker, calls *[]MarkerCall, warnings *[]string) {
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
					} else {
						line := shimscanner.GetECMALineOfPosition(sf, node.Pos())
						*warnings = append(*warnings, fmt.Sprintf(
							"%s:%d — %s<T>() type argument is not a named type; call left as a no-op (validation skipped). Use a named interface or type alias.",
							sf.FileName(), line+1, origName))
					}
				}
			}
		}
	}

	// Recurse into children
	node.ForEachChild(func(child *ast.Node) bool {
		walkNode(sf, child, importedNames, checker, calls, warnings)
		return false // continue visiting
	})
}

// isAnonymousTypeName reports checker-internal symbol names that never
// correspond to a declared type the companion generator can find.
func isAnonymousTypeName(name string) bool {
	return name == "" || name == "__type" || name == "__object" || strings.HasPrefix(name, "\xfe")
}

// resolveTypeArgName resolves a type argument node to a named type string.
//
// The syntactic reference name is preferred: for `assert<Foo>(x)` where Foo is
// a type alias to an object literal, the resolved type's symbol is the
// anonymous `__type`, but the reference identifier still names the alias.
// Import aliases are skipped so `import { Foo as Bar }` resolves to "Foo".
func resolveTypeArgName(typeNode *ast.Node, checker *shimchecker.Checker) string {
	if typeNode.Kind == ast.KindTypeReference {
		ref := typeNode.AsTypeReferenceNode()
		if ref.TypeName != nil && ref.TypeName.Kind == ast.KindIdentifier {
			if sym := checker.GetSymbolAtLocation(ref.TypeName); sym != nil {
				if resolved := shimchecker.SkipAlias(sym, checker); resolved != nil && !isAnonymousTypeName(resolved.Name) {
					return resolved.Name
				}
			}
		}
	}

	resolvedType := checker.GetTypeFromTypeNode(typeNode)
	if resolvedType == nil {
		return ""
	}
	sym := resolvedType.Symbol()
	if sym == nil || isAnonymousTypeName(sym.Name) {
		return ""
	}
	return sym.Name
}
