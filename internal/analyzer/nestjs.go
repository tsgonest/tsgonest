// Package analyzer provides AST and type analysis utilities for tsgonest.
// This file implements NestJS decorator recognition via static AST analysis.
package analyzer

import (
	"strconv"
	"strings"

	"github.com/microsoft/typescript-go/shim/ast"
)

// DecoratorInfo holds the parsed information from a decorator.
type DecoratorInfo struct {
	// Name is the decorator function name (e.g., "Controller", "Get", "Body").
	Name string
	// Args holds the string arguments to the decorator call (e.g., ["users"] for @Controller("users")).
	Args []string
	// NumericArg holds a numeric argument if present (e.g., 201 for @HttpCode(201)).
	NumericArg *float64
	// TypeArgNodes holds AST nodes of the type arguments from the call expression.
	// e.g., for @Returns<UserDto>(), TypeArgNodes contains the TypeReference node for UserDto.
	TypeArgNodes []*ast.Node
	// ObjectLiteralArg holds the first object literal argument's property name→value AST nodes.
	// e.g., for @Returns<Buffer>({ contentType: 'application/pdf' }), this maps "contentType" → StringLiteral("application/pdf").
	ObjectLiteralArg map[string]*ast.Node
}

// ParseDecorator extracts the decorator name and arguments from a Decorator AST node.
// Returns nil if the decorator cannot be parsed.
func ParseDecorator(node *ast.Node) *DecoratorInfo {
	if node.Kind != ast.KindDecorator {
		return nil
	}
	dec := node.AsDecorator()
	expr := dec.Expression

	switch expr.Kind {
	case ast.KindIdentifier:
		// Simple decorator without parentheses: @Injectable
		return &DecoratorInfo{Name: expr.AsIdentifier().Text}

	case ast.KindCallExpression:
		// Decorator with call: @Controller('users'), @Get(':id'), @HttpCode(201)
		call := expr.AsCallExpression()
		name := getDecoratorExprName(call.Expression)
		if name == "" {
			return nil
		}

		info := &DecoratorInfo{Name: name}

		// Extract type arguments (e.g., @Returns<UserDto>() → TypeArguments = [TypeReference(UserDto)])
		if call.TypeArguments != nil {
			for _, typeArg := range call.TypeArguments.Nodes {
				info.TypeArgNodes = append(info.TypeArgNodes, typeArg)
			}
		}

		// Extract arguments
		if call.Arguments != nil {
			for _, arg := range call.Arguments.Nodes {
				switch arg.Kind {
				case ast.KindStringLiteral:
					info.Args = append(info.Args, arg.AsStringLiteral().Text)
				case ast.KindNumericLiteral:
					text := arg.Text()
					if num, err := strconv.ParseFloat(text, 64); err == nil {
						info.NumericArg = &num
					}
				case ast.KindNoSubstitutionTemplateLiteral:
					info.Args = append(info.Args, arg.Text())
				case ast.KindObjectLiteralExpression:
					// Extract object literal properties (e.g., { contentType: 'application/pdf' })
					info.ObjectLiteralArg = extractObjectLiteralProps(arg)
				}
			}
		}

		return info

	case ast.KindPropertyAccessExpression:
		// Property access decorator without call: @nestjs.Injectable
		pa := expr.AsPropertyAccessExpression()
		name := pa.Name().AsIdentifier().Text
		return &DecoratorInfo{Name: name}
	}

	return nil
}

// extractObjectLiteralProps extracts string property assignments from an object literal expression.
// Returns a map of property name → value AST node.
// Handles PropertyAssignment nodes with identifier or string literal names.
func extractObjectLiteralProps(objLit *ast.Node) map[string]*ast.Node {
	if objLit.Kind != ast.KindObjectLiteralExpression {
		return nil
	}
	ole := objLit.AsObjectLiteralExpression()
	if ole.Properties == nil {
		return nil
	}
	props := make(map[string]*ast.Node)
	for _, prop := range ole.Properties.Nodes {
		if prop.Kind != ast.KindPropertyAssignment {
			continue
		}
		pa := prop.AsPropertyAssignment()
		if pa.Name() == nil {
			continue
		}
		var name string
		switch pa.Name().Kind {
		case ast.KindIdentifier:
			name = pa.Name().AsIdentifier().Text
		case ast.KindStringLiteral:
			name = pa.Name().AsStringLiteral().Text
		default:
			continue
		}
		props[name] = pa.Initializer
	}
	return props
}

// getDecoratorExprName extracts the function name from a decorator call expression's callee.
func getDecoratorExprName(expr *ast.Node) string {
	switch expr.Kind {
	case ast.KindIdentifier:
		return expr.AsIdentifier().Text
	case ast.KindPropertyAccessExpression:
		return expr.AsPropertyAccessExpression().Name().AsIdentifier().Text
	}
	return ""
}

// IsControllerDecorator checks if a decorator is @Controller().
func IsControllerDecorator(info *DecoratorInfo) bool {
	return info != nil && info.Name == "Controller"
}

// GetControllerPath extracts the path from @Controller('path').
// Returns empty string if no path argument.
func GetControllerPath(info *DecoratorInfo) string {
	if !IsControllerDecorator(info) {
		return ""
	}
	if len(info.Args) > 0 {
		return cleanPath(info.Args[0])
	}
	return ""
}

// CombinePaths joins a controller prefix and method sub-path into a full route path.
// Handles leading/trailing slashes.
func CombinePaths(prefix, subPath string) string {
	prefix = cleanPath(prefix)
	subPath = cleanPath(subPath)

	if prefix == "" && subPath == "" {
		return "/"
	}
	if prefix == "" {
		return "/" + subPath
	}
	if subPath == "" {
		return "/" + prefix
	}
	return "/" + prefix + "/" + subPath
}

// cleanPath removes leading and trailing slashes.
func cleanPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	return p
}
