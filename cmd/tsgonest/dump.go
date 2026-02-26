package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/microsoft/typescript-go/shim/core"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// metadataDump is the JSON output structure for --dump-metadata.
type metadataDump struct {
	Files    []fileDump                    `json:"files"`
	Registry map[string]*metadata.Metadata `json:"registry"`
}

type fileDump struct {
	FileName string                       `json:"fileName"`
	Types    map[string]metadata.Metadata `json:"types"`
}

// runDumpMetadata extracts type metadata from all non-declaration source files
// and outputs it as JSON to stdout.
func runDumpMetadata(program *shimcompiler.Program, opts *core.CompilerOptions) int {
	checker, release := shimcompiler.Program_GetTypeChecker(program, context.Background())
	if checker == nil {
		fmt.Fprintln(os.Stderr, "error: could not get type checker")
		return 1
	}
	defer release()

	walker := analyzer.NewTypeWalker(checker)
	if opts.ExactOptionalPropertyTypes == core.TSTrue {
		walker.SetExactOptionalPropertyTypes(true)
	}

	var files []fileDump
	for _, sf := range program.GetSourceFiles() {
		if sf.IsDeclarationFile {
			continue
		}

		types := make(map[string]metadata.Metadata)

		for _, stmt := range sf.Statements.Nodes {
			switch stmt.Kind {
			case ast.KindTypeAliasDeclaration:
				decl := stmt.AsTypeAliasDeclaration()
				name := decl.Name().Text()
				resolvedType := shimchecker.Checker_getTypeFromTypeNode(checker, decl.Type)
				m := walker.WalkNamedType(name, resolvedType)
				types[name] = m

			case ast.KindInterfaceDeclaration:
				decl := stmt.AsInterfaceDeclaration()
				name := decl.Name().Text()
				sym := checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(checker, sym)
					types[name] = walker.WalkType(resolvedType)
				}

			case ast.KindClassDeclaration:
				decl := stmt.AsClassDeclaration()
				if decl.Name() != nil {
					name := decl.Name().Text()
					sym := checker.GetSymbolAtLocation(decl.Name())
					if sym != nil {
						resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(checker, sym)
						types[name] = walker.WalkType(resolvedType)
					}
				}

			case ast.KindEnumDeclaration:
				decl := stmt.AsEnumDeclaration()
				name := decl.Name().Text()
				sym := checker.GetSymbolAtLocation(decl.Name())
				if sym != nil {
					resolvedType := shimchecker.Checker_getDeclaredTypeOfSymbol(checker, sym)
					types[name] = walker.WalkType(resolvedType)
				}
			}
		}

		if len(types) > 0 {
			files = append(files, fileDump{
				FileName: sf.FileName(),
				Types:    types,
			})
		}
	}

	dump := metadataDump{
		Files:    files,
		Registry: walker.Registry().Types,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(dump); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
		return 1
	}
	return 0
}

// hashFileContent returns the hex-encoded SHA256 hash of a file's content.
// Returns an empty string if the file cannot be read.
func hashFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
