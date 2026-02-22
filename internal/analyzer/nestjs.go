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
