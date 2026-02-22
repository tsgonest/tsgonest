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
