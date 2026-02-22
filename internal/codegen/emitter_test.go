package codegen

import (
	"strings"
	"testing"
)

func TestEmitterLine(t *testing.T) {
	e := NewEmitter()
	e.Line("const x = 1;")
	if got := e.String(); got != "const x = 1;\n" {
		t.Errorf("got %q", got)
	}
}

func TestEmitterBlank(t *testing.T) {
	e := NewEmitter()
	e.Line("a")
	e.Blank()
	e.Line("b")
	if got := e.String(); got != "a\n\nb\n" {
		t.Errorf("got %q", got)
	}
}

func TestEmitterBlock(t *testing.T) {
	e := NewEmitter()
	e.Block("if (true)")
	e.Line("return 1;")
	e.EndBlock()
	expected := "if (true) {\n  return 1;\n}\n"
	if got := e.String(); got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestEmitterNestedBlocks(t *testing.T) {
	e := NewEmitter()
	e.Block("function foo()")
	e.Block("if (x)")
	e.Line("return;")
	e.EndBlock()
	e.EndBlock()
	expected := "function foo() {\n  if (x) {\n    return;\n  }\n}\n"
	if got := e.String(); got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestEmitterFormat(t *testing.T) {
	e := NewEmitter()
	e.Line("const %s = %d;", "x", 42)
	if got := e.String(); got != "const x = 42;\n" {
		t.Errorf("got %q", got)
	}
}

func TestEmitterEndBlockSuffix(t *testing.T) {
	e := NewEmitter()
	e.Block("if (a)")
	e.Line("return 1;")
	e.EndBlockSuffix(" else {")
	e.indent++
	e.Line("return 2;")
	e.indent--
	e.Line("}")
	got := e.String()
	if !strings.Contains(got, "} else {") {
		t.Errorf("expected '} else {' in output, got %q", got)
	}
}
