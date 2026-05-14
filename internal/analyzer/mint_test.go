package analyzer_test

import (
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
)

func TestIsMintkitModule(t *testing.T) {
	cases := []struct {
		spec string
		want bool
	}{
		{"@mintkit/core", true},
		{"@mintkit/runtime", true},
		{"@mintkit/x", true},
		{"@mintkit", false},
		{"@mintkit/", false},
		{"mintkit", false},
		{"@nestjs/common", false},
		{"", false},
	}
	for _, c := range cases {
		if got := analyzer.IsMintkitModule(c.spec); got != c.want {
			t.Errorf("IsMintkitModule(%q) = %v, want %v", c.spec, got, c.want)
		}
	}
}

func TestControllerAnalyzer_MintFrameworkDetection(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"node_modules/@mintkit/core/index.d.ts": `
			export function Controller(path: string): ClassDecorator;
			export function Get(path?: string): MethodDecorator;
		`,
		"hello.controller.ts": `
			import { Controller, Get } from "@mintkit/core";

			@Controller("/hello")
			export class HelloController {
				@Get()
				hello(): string { return "hi"; }
			}
		`,
	}, "hello.controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	ctrl := controllers[0]
	if ctrl.Name != "HelloController" {
		t.Errorf("expected Name=HelloController, got %q", ctrl.Name)
	}
	if ctrl.Path != "hello" {
		t.Errorf("expected Path=hello, got %q", ctrl.Path)
	}
	if ctrl.Framework != "mint" {
		t.Errorf("expected Framework=mint, got %q", ctrl.Framework)
	}
	if len(ctrl.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(ctrl.Routes))
	}
	if ctrl.Routes[0].Method != "GET" {
		t.Errorf("expected Method=GET, got %q", ctrl.Routes[0].Method)
	}
}

func TestControllerAnalyzer_NestJSFrameworkUnset(t *testing.T) {
	env := setupWalkerMultiFile(t, map[string]string{
		"node_modules/@nestjs/common/index.d.ts": `
			export function Controller(path: string): ClassDecorator;
			export function Get(path?: string): MethodDecorator;
		`,
		"hello.controller.ts": `
			import { Controller, Get } from "@nestjs/common";

			@Controller("/hello")
			export class HelloController {
				@Get()
				hello(): string { return "hi"; }
			}
		`,
	}, "hello.controller.ts")
	defer env.release()

	ca, caRelease := analyzer.NewControllerAnalyzer(env.program)
	defer caRelease()

	controllers := ca.AnalyzeSourceFile(env.sourceFile)
	if len(controllers) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(controllers))
	}
	ctrl := controllers[0]
	if ctrl.Framework != "" {
		t.Errorf("expected empty Framework for NestJS, got %q", ctrl.Framework)
	}
	if ctrl.Path != "hello" {
		t.Errorf("expected Path=hello, got %q", ctrl.Path)
	}
	if len(ctrl.Routes) != 1 || ctrl.Routes[0].Method != "GET" {
		t.Fatalf("expected 1 GET route")
	}
}
