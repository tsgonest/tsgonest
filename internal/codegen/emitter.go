// Package codegen generates JavaScript companion files (validation, serialization)
// from type metadata.
package codegen

import (
	"fmt"
	"strings"
)

// Emitter builds JavaScript source code with proper indentation.
type Emitter struct {
	buf    strings.Builder
	indent int
}

// NewEmitter creates a new JavaScript code emitter.
func NewEmitter() *Emitter {
	return &Emitter{}
}

// Line writes a single line of code at the current indentation level.
func (e *Emitter) Line(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if line == "" {
		e.buf.WriteByte('\n')
		return
	}
	for i := 0; i < e.indent; i++ {
		e.buf.WriteString("  ")
	}
	e.buf.WriteString(line)
	e.buf.WriteByte('\n')
}

// Raw writes a raw string without indentation or newline.
func (e *Emitter) Raw(s string) {
	e.buf.WriteString(s)
}

// Blank writes an empty line.
func (e *Emitter) Blank() {
	e.buf.WriteByte('\n')
}

// Block opens a block (appends " {" to the line and increases indent).
func (e *Emitter) Block(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	for i := 0; i < e.indent; i++ {
		e.buf.WriteString("  ")
	}
	e.buf.WriteString(line)
	e.buf.WriteString(" {\n")
	e.indent++
}

// EndBlock closes a block (decreases indent and writes "}").
func (e *Emitter) EndBlock() {
	e.indent--
	for i := 0; i < e.indent; i++ {
		e.buf.WriteString("  ")
	}
	e.buf.WriteString("}\n")
}

// EndBlockSuffix closes a block with a suffix (e.g., "} else {").
func (e *Emitter) EndBlockSuffix(suffix string) {
	e.indent--
	for i := 0; i < e.indent; i++ {
		e.buf.WriteString("  ")
	}
	e.buf.WriteString("}")
	e.buf.WriteString(suffix)
	e.buf.WriteByte('\n')
}

// Indent increases the indentation level.
func (e *Emitter) Indent() {
	e.indent++
}

// Dedent decreases the indentation level.
func (e *Emitter) Dedent() {
	if e.indent > 0 {
		e.indent--
	}
}

// String returns the accumulated source code.
func (e *Emitter) String() string {
	return e.buf.String()
}

// Len returns the current byte length.
func (e *Emitter) Len() int {
	return e.buf.Len()
}

// ConvertToCommonJS converts ESM-style generated code to CommonJS format.
// This transforms:
//   - `import { A, B } from "path";` → `const { A, B } = require("path");`
//   - `export function foo(` → `function foo(`
//   - `export const foo =` → `const foo =`
//   - `export declare function ...` → `declare function ...` (d.ts files, kept as-is)
//
// and appends `module.exports = { foo, bar, ... };` with all collected exports.
func ConvertToCommonJS(esm string) string {
	lines := strings.Split(esm, "\n")
	var out []string
	var exports []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// import { A, B } from "path"; → const { A, B } = require("path");
		if strings.HasPrefix(trimmed, "import ") && strings.Contains(trimmed, " from ") {
			// Extract: import { names } from "module";
			fromIdx := strings.Index(trimmed, " from ")
			if fromIdx > 0 {
				importPart := trimmed[len("import "):fromIdx]
				modulePart := trimmed[fromIdx+len(" from "):]
				modulePart = strings.TrimSuffix(modulePart, ";")
				// Preserve leading whitespace
				leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				out = append(out, fmt.Sprintf("%sconst %s = require(%s);", leading, importPart, modulePart))
				continue
			}
		}

		// export function foo( → function foo(
		if strings.HasPrefix(trimmed, "export function ") {
			fnName := extractExportName(trimmed[len("export function "):])
			if fnName != "" {
				exports = append(exports, fnName)
			}
			leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			out = append(out, leading+strings.TrimPrefix(trimmed, "export "))
			continue
		}

		// export const foo = → const foo =
		if strings.HasPrefix(trimmed, "export const ") {
			constName := extractExportName(trimmed[len("export const "):])
			if constName != "" {
				exports = append(exports, constName)
			}
			leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			out = append(out, leading+strings.TrimPrefix(trimmed, "export "))
			continue
		}

		// export class Foo → class Foo
		if strings.HasPrefix(trimmed, "export class ") {
			className := extractExportName(trimmed[len("export class "):])
			if className != "" {
				exports = append(exports, className)
			}
			leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			out = append(out, leading+strings.TrimPrefix(trimmed, "export "))
			continue
		}

		out = append(out, line)
	}

	result := strings.Join(out, "\n")

	// Append module.exports
	if len(exports) > 0 {
		if !strings.HasSuffix(result, "\n") {
			result += "\n"
		}
		result += "module.exports = { " + strings.Join(exports, ", ") + " };\n"
	}

	return result
}

// ConvertDtsToCommonJS converts ESM-style .d.ts content to CommonJS-compatible declarations.
// Transforms `export declare` → `declare` and adds `export = { ... }` at the end.
func ConvertDtsToCommonJS(esm string) string {
	lines := strings.Split(esm, "\n")
	var out []string
	var exports []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// export declare function foo( → declare function foo(
		if strings.HasPrefix(trimmed, "export declare function ") {
			fnName := extractExportName(trimmed[len("export declare function "):])
			if fnName != "" {
				exports = append(exports, fnName)
			}
			leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			out = append(out, leading+strings.TrimPrefix(trimmed, "export "))
			continue
		}

		// export declare const foo: → declare const foo:
		if strings.HasPrefix(trimmed, "export declare const ") {
			constName := extractExportName(trimmed[len("export declare const "):])
			if constName != "" {
				exports = append(exports, constName)
			}
			leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			out = append(out, leading+strings.TrimPrefix(trimmed, "export "))
			continue
		}

		// export declare class Foo → declare class Foo
		if strings.HasPrefix(trimmed, "export declare class ") {
			className := extractExportName(trimmed[len("export declare class "):])
			if className != "" {
				exports = append(exports, className)
			}
			leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			out = append(out, leading+strings.TrimPrefix(trimmed, "export "))
			continue
		}

		out = append(out, line)
	}

	result := strings.Join(out, "\n")

	if len(exports) > 0 {
		if !strings.HasSuffix(result, "\n") {
			result += "\n"
		}
		result += "export { " + strings.Join(exports, ", ") + " };\n"
	}

	return result
}

// extractExportName extracts the identifier name from a string like "foo(" or "foo =" or "foo:".
func extractExportName(s string) string {
	var name strings.Builder
	for _, c := range s {
		if c == '(' || c == ' ' || c == '=' || c == ':' || c == '<' {
			break
		}
		name.WriteRune(c)
	}
	return name.String()
}
