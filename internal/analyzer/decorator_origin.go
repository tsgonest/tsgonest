package analyzer

import (
	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
)

// DecoratorOrigin contains the resolved original name and import source of a decorator.
type DecoratorOrigin struct {
	// Name is the original exported name (e.g., "Returns", "Body", "Get").
	// For aliased imports like `import { Body as B }`, this is "Body", not "B".
	// For namespace imports like `import * as nest from '...'` with @nest.Get(),
	// this is "Get" (the property name).
	Name string
	// ModuleSpecifier is the import module path (e.g., "@tsgonest/runtime", "@nestjs/common").
	// Empty if the decorator is locally defined (not imported).
	ModuleSpecifier string
}

// ResolveDecoratorOrigin resolves the original name and import source module of a decorator.
// Handles:
//   - Direct imports:    import { Returns } from '@tsgonest/runtime'   → Name="Returns", Module="@tsgonest/runtime"
//   - Aliased imports:   import { Returns as R } from '@tsgonest/runtime' → Name="Returns", Module="@tsgonest/runtime"
//   - Namespace imports: import * as rt from '@tsgonest/runtime'; @rt.Returns() → Name="Returns", Module="@tsgonest/runtime"
//
// Returns nil if the decorator origin cannot be determined (e.g., locally defined function).
func ResolveDecoratorOrigin(dec *ast.Node, checker *shimchecker.Checker) *DecoratorOrigin {
	if dec.Kind != ast.KindDecorator {
		return nil
	}
	expr := dec.AsDecorator().Expression

	// Unwrap call expression to get the callee: @Foo() → Foo, @ns.Foo() → ns.Foo
	callee := expr
	if callee.Kind == ast.KindCallExpression {
		callee = callee.AsCallExpression().Expression
	}

	switch callee.Kind {
	case ast.KindIdentifier:
		// Direct or aliased import: @Returns() or @R() where R is aliased from Returns
		return resolveIdentifierOrigin(callee, checker)

	case ast.KindPropertyAccessExpression:
		// Namespace import: @nest.Get() or @rt.Returns()
		return resolvePropertyAccessOrigin(callee, checker)
	}

	return nil
}

// resolveIdentifierOrigin resolves a bare identifier decorator like @Returns() or @R().
// Returns the original name and module specifier if it was imported, nil otherwise.
func resolveIdentifierOrigin(ident *ast.Node, checker *shimchecker.Checker) *DecoratorOrigin {
	sym := checker.GetSymbolAtLocation(ident)
	if sym == nil {
		return nil
	}

	// Non-alias: locally defined decorator. We can still return the name but no module.
	if sym.Flags&ast.SymbolFlagsAlias == 0 {
		return nil
	}

	// Resolve the alias to get the original exported name
	original := checker.GetAliasedSymbol(sym)
	originalName := sym.Name // default to local name
	if original != nil && original.Name != "" {
		originalName = original.Name
	}

	// Walk the alias symbol's declaration to find the import module specifier
	moduleSpec := moduleSpecifierFromDeclarations(sym.Declarations)

	return &DecoratorOrigin{
		Name:            originalName,
		ModuleSpecifier: moduleSpec,
	}
}

// resolvePropertyAccessOrigin resolves a namespace-qualified decorator like @nest.Get().
// The property name ("Get") is the original name, and the namespace object ("nest")
// traces back to the import declaration's module specifier.
func resolvePropertyAccessOrigin(pa *ast.Node, checker *shimchecker.Checker) *DecoratorOrigin {
	propAccess := pa.AsPropertyAccessExpression()

	// The property name is the original exported name (e.g., "Get" from @nest.Get())
	propName := propAccess.Name().AsIdentifier().Text

	// The object expression should be the namespace identifier (e.g., "nest")
	obj := propAccess.Expression
	if obj.Kind != ast.KindIdentifier {
		return nil
	}

	// Resolve the namespace symbol
	nsSym := checker.GetSymbolAtLocation(obj)
	if nsSym == nil {
		return nil
	}

	// The namespace symbol should be an alias (from import * as ns)
	if nsSym.Flags&ast.SymbolFlagsAlias == 0 {
		return nil
	}

	// Walk the namespace symbol's declaration to find the import module specifier.
	// For `import * as nest from '...'`, the declaration is a NamespaceImport node.
	moduleSpec := moduleSpecifierFromDeclarations(nsSym.Declarations)

	return &DecoratorOrigin{
		Name:            propName,
		ModuleSpecifier: moduleSpec,
	}
}

// moduleSpecifierFromDeclarations walks the declaration nodes of an import symbol
// to find the ImportDeclaration and extract its module specifier string.
//
// Parent chains:
//   - ImportSpecifier → NamedImports → ImportClause → ImportDeclaration
//   - NamespaceImport → ImportClause → ImportDeclaration
//   - ImportClause → ImportDeclaration
func moduleSpecifierFromDeclarations(declarations []*ast.Node) string {
	for _, decl := range declarations {
		if spec := moduleSpecifierFromNode(decl); spec != "" {
			return spec
		}
	}
	return ""
}

// moduleSpecifierFromNode walks up from an import-related node to the ImportDeclaration
// and returns the module specifier text.
func moduleSpecifierFromNode(node *ast.Node) string {
	// Determine how many Parent hops to reach ImportDeclaration
	var importDecl *ast.Node
	switch node.Kind {
	case ast.KindImportSpecifier:
		// ImportSpecifier → NamedImports → ImportClause → ImportDeclaration
		if node.Parent != nil && node.Parent.Parent != nil && node.Parent.Parent.Parent != nil {
			importDecl = node.Parent.Parent.Parent
		}
	case ast.KindNamespaceImport:
		// NamespaceImport → ImportClause → ImportDeclaration
		if node.Parent != nil && node.Parent.Parent != nil {
			importDecl = node.Parent.Parent
		}
	case ast.KindImportClause:
		// ImportClause → ImportDeclaration
		if node.Parent != nil {
			importDecl = node.Parent
		}
	default:
		// Walk up generically looking for ImportDeclaration
		for n := node.Parent; n != nil; n = n.Parent {
			if n.Kind == ast.KindImportDeclaration {
				importDecl = n
				break
			}
		}
	}

	if importDecl == nil || importDecl.Kind != ast.KindImportDeclaration {
		return ""
	}

	modSpec := importDecl.AsImportDeclaration().ModuleSpecifier
	if modSpec == nil || modSpec.Kind != ast.KindStringLiteral {
		return ""
	}

	return modSpec.AsStringLiteral().Text
}

// IsTsgonestModule checks if a module specifier refers to a tsgonest package.
// Matches "tsgonest", "@tsgonest/runtime", or any "@tsgonest/*" scoped package.
func IsTsgonestModule(moduleSpecifier string) bool {
	return moduleSpecifier == "tsgonest" ||
		moduleSpecifier == "@tsgonest/runtime" ||
		len(moduleSpecifier) > 10 && moduleSpecifier[:10] == "@tsgonest/"
}
