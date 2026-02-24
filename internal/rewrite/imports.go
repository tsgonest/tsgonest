package rewrite

import (
	"fmt"
	"path/filepath"
	"strings"
)

// companionFuncName returns the companion function name for a marker+type combination.
// e.g., ("is", "CreateUserDto") → "isCreateUserDto"
// e.g., ("validate", "CreateUserDto") → "validateCreateUserDto"
func companionFuncName(markerName, typeName string) string {
	return markerName + typeName
}

// companionRelativePath computes the relative import path from an output JS file
// to its companion file.
// outputFile: "/abs/dist/user/user.controller.js"
// companionPath: "/abs/dist/user/user.dto.CreateUserDto.tsgonest.js"
// → "./user.dto.CreateUserDto.tsgonest.js"
func companionRelativePath(fromFile, companionPath string) string {
	fromDir := filepath.Dir(fromFile)
	rel, err := filepath.Rel(fromDir, companionPath)
	if err != nil {
		// Fallback to the full path
		return companionPath
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, ".") {
		rel = "./" + rel
	}
	return rel
}

// generateESMImport generates an ESM import statement.
// e.g., import { isCreateUserDto, assertCreateUserDto } from "./user.dto.CreateUserDto.tsgonest.js";
func generateESMImport(funcNames []string, importPath string) string {
	return fmt.Sprintf("import { %s } from %q;", strings.Join(funcNames, ", "), importPath)
}

// generateCJSRequire generates a CJS require statement.
// e.g., const { isCreateUserDto, assertCreateUserDto } = require("./user.dto.CreateUserDto.tsgonest.js");
func generateCJSRequire(funcNames []string, importPath string) string {
	return fmt.Sprintf("const { %s } = require(%q);", strings.Join(funcNames, ", "), importPath)
}

// companionImports groups marker calls by companion file and generates import statements.
// Returns a list of import/require lines to insert at the top of the file.
func companionImports(calls []MarkerCall, companionMap map[string]string, outputFile string, moduleFormat string) []string {
	// Group function names by companion file path
	type companionGroup struct {
		path      string
		funcNames []string
	}
	groups := make(map[string]*companionGroup) // keyed by companion abs path
	order := []string{}                        // preserve ordering

	for _, call := range calls {
		companionPath, ok := companionMap[call.TypeName]
		if !ok {
			continue
		}
		g, exists := groups[companionPath]
		if !exists {
			g = &companionGroup{path: companionPath}
			groups[companionPath] = g
			order = append(order, companionPath)
		}
		funcName := companionFuncName(call.FunctionName, call.TypeName)
		// Deduplicate function names within the same companion
		found := false
		for _, existing := range g.funcNames {
			if existing == funcName {
				found = true
				break
			}
		}
		if !found {
			g.funcNames = append(g.funcNames, funcName)
		}
	}

	var imports []string
	for _, absPath := range order {
		g := groups[absPath]
		relPath := companionRelativePath(outputFile, g.path)
		if moduleFormat == "cjs" {
			imports = append(imports, generateCJSRequire(g.funcNames, relPath))
		} else {
			imports = append(imports, generateESMImport(g.funcNames, relPath))
		}
	}

	return imports
}
