package analyzer_test

import (
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
)

func TestEdgeCases_Analyzer_CombinePaths(t *testing.T) {
	t.Run("Edge11_combine_paths_duplicate_slashes", func(t *testing.T) {
		if got := analyzer.CombinePaths("//api//", "//users//"); got != "/api/users" {
			t.Fatalf("expected duplicate slashes to be normalized, got %q", got)
		}
	})

	t.Run("Edge12_combine_paths_trim_whitespace", func(t *testing.T) {
		if got := analyzer.CombinePaths(" users ", " :id "); got != "/users/:id" {
			t.Fatalf("expected whitespace-only path padding to be trimmed, got %q", got)
		}
	})

	t.Run("Edge15_combine_paths_all_slashes_should_be_root", func(t *testing.T) {
		if got := analyzer.CombinePaths("///", "///"); got != "/" {
			t.Fatalf("expected root path for slash-only inputs, got %q", got)
		}
	})
}

func TestEdgeCases_Analyzer_Decorators(t *testing.T) {
	t.Run("Edge16_parse_decorator_negative_numeric_arg", func(t *testing.T) {
		env := setupWalker(t, `
			function HttpCode(code: number): MethodDecorator { return (t, k, d) => d; }
			export class C {
				@HttpCode(-1)
				m(): void {}
			}
		`)
		defer env.release()

		method := findMethodInSourceFile(t, env.sourceFile, "C", "m")
		info := analyzer.ParseDecorator(method.Decorators()[0])
		if info == nil || info.NumericArg == nil || *info.NumericArg != -1 {
			t.Fatalf("expected ParseDecorator to extract negative numeric arg -1, got %#v", info)
		}
	})

	t.Run("Edge17_parse_decorator_positive_unary_numeric_arg", func(t *testing.T) {
		env := setupWalker(t, `
			function HttpCode(code: number): MethodDecorator { return (t, k, d) => d; }
			export class C {
				@HttpCode(+201)
				m(): void {}
			}
		`)
		defer env.release()

		method := findMethodInSourceFile(t, env.sourceFile, "C", "m")
		info := analyzer.ParseDecorator(method.Decorators()[0])
		if info == nil || info.NumericArg == nil || *info.NumericArg != 201 {
			t.Fatalf("expected ParseDecorator to extract unary +201, got %#v", info)
		}
	})

	t.Run("Edge18_parse_decorator_shorthand_object_property", func(t *testing.T) {
		env := setupWalker(t, `
			function Returns(opts: { contentType: string }): MethodDecorator { return (t, k, d) => d; }
			const contentType = "application/json";
			export class C {
				@Returns({ contentType })
				m(): void {}
			}
		`)
		defer env.release()

		method := findMethodInSourceFile(t, env.sourceFile, "C", "m")
		info := analyzer.ParseDecorator(method.Decorators()[0])
		if info == nil || info.ObjectLiteralArg == nil || info.ObjectLiteralArg["contentType"] == nil {
			t.Fatalf("expected ParseDecorator to capture shorthand object literal props, got %#v", info)
		}
	})

	t.Run("Edge19_parse_decorator_boolean_literal_arg", func(t *testing.T) {
		env := setupWalker(t, `
			function Flag(v: boolean): MethodDecorator { return (t, k, d) => d; }
			export class C {
				@Flag(true)
				m(): void {}
			}
		`)
		defer env.release()

		method := findMethodInSourceFile(t, env.sourceFile, "C", "m")
		info := analyzer.ParseDecorator(method.Decorators()[0])
		if info == nil || len(info.Args) == 0 || info.Args[0] != "true" {
			t.Fatalf("expected ParseDecorator to preserve boolean args, got %#v", info)
		}
	})

}
