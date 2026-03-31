package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/compiler"
	"github.com/tsgonest/tsgonest/internal/config"
	"github.com/tsgonest/tsgonest/internal/openapi"
)

// runOpenAPI generates OpenAPI documents without a full build.
// Skips: emit, companion files, SDK generation, asset copying.
func runOpenAPI(args []string) int {
	fs := flag.NewFlagSet("openapi", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to tsgonest config file")
	tsconfigPath := fs.String("project", "tsconfig.json", "Path to tsconfig.json")
	fs.StringVar(tsconfigPath, "p", "tsconfig.json", "Path to tsconfig.json (shorthand)")
	outputOverride := fs.String("output", "", "Override output path (single-output mode)")
	nameFilter := fs.String("name", "", "Generate only the named output (multi-output mode)")
	noCheck := fs.Bool("no-check", false, "Skip type checking (syntax errors still reported)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	start := time.Now()
	pretty := compiler.IsPrettyOutput()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not get working directory: %v\n", err)
		return 1
	}

	// Load config
	cfgResult, err := loadOrDiscoverConfig(*configPath, cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	cfg := cfgResult.Config
	configDir := cfgResult.Dir
	if cfgResult.Path != "" {
		printStatus(os.Stderr, pretty, "◆", "loaded config from %s", filepath.Base(cfgResult.Path))
	}
	if cfg == nil {
		fmt.Fprintln(os.Stderr, "error: no tsgonest config found")
		return 1
	}
	if len(cfg.OpenAPIOutputs) == 0 {
		fmt.Fprintln(os.Stderr, "error: no OpenAPI outputs configured")
		return 1
	}

	// Filter outputs by --name
	outputs := cfg.OpenAPIOutputs
	if *nameFilter != "" {
		var found bool
		for _, o := range outputs {
			if o.Name == *nameFilter {
				outputs = []config.OpenAPIOutputConfig{o}
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "error: no OpenAPI output named %q\n", *nameFilter)
			return 1
		}
	}

	// Apply --output override (only valid for single output)
	if *outputOverride != "" {
		if len(outputs) > 1 {
			fmt.Fprintln(os.Stderr, "error: --output cannot be used with multiple OpenAPI outputs; use --name to select one")
			return 1
		}
		outputs[0].Output = *outputOverride
	}

	// Parse tsconfig
	tsFS := compiler.CreateDefaultFS()
	host := compiler.CreateDefaultHost(cwd, tsFS)
	printStatus(os.Stderr, pretty, "◆", "parsing tsconfig: %s", *tsconfigPath)

	parsedConfig, diags, err := compiler.ParseTSConfig(tsFS, cwd, *tsconfigPath, host, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(diags) > 0 {
		fmt.Fprint(os.Stderr, compiler.FormatDiagnostics(diags))
		return 1
	}

	// Create program
	program, programDiags, err := compiler.CreateProgramFromConfig(false, parsedConfig, host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(programDiags) > 0 {
		fmt.Fprint(os.Stderr, compiler.FormatDiagnostics(programDiags))
	}

	// Gather diagnostics (default: --check)
	diagStart := time.Now()
	allDiags := compiler.GatherDiagnostics(program, *noCheck)
	if len(allDiags) > 0 {
		reportDiag := compiler.CreateDiagnosticReporter(os.Stderr, cwd, pretty)
		for _, d := range allDiags {
			reportDiag(d)
		}
	}
	hasErrors := compiler.CountErrors(allDiags) > 0
	printStatus(os.Stderr, pretty, "◆", "diagnostics: %s", time.Since(diagStart).Round(time.Millisecond))
	if hasErrors {
		return 1
	}

	// Get type checker for controller analysis
	sharedChecker, release := program.GetTypeChecker(context.Background())
	if sharedChecker == nil {
		fmt.Fprintln(os.Stderr, "error: could not get type checker")
		return 1
	}
	defer release()

	sharedWalker := analyzer.NewTypeWalker(sharedChecker)

	// Analyze controllers
	ca := analyzer.NewControllerAnalyzerWithWalker(program, sharedChecker, sharedWalker)
	controllers := ca.AnalyzeProgram(cfg.Controllers.Include, cfg.Controllers.Exclude)
	controllerRegistry := ca.Registry()
	printStatus(os.Stderr, pretty, "✓", "analyzed %d controller(s)", len(controllers))

	if len(controllers) == 0 {
		fmt.Fprintln(os.Stderr, "warning: no controllers found matching include patterns")
		return 0
	}

	// Generate each output
	for i := range outputs {
		outputCfg := &outputs[i]
		if outputCfg.Output == "" {
			continue
		}

		// Post-filter controllers for this output
		filtered := controllers
		if outputCfg.Controllers != nil || len(outputCfg.IncludeTags) > 0 || len(outputCfg.ExcludeTags) > 0 {
			filterOpts := openapi.FilterOptions{
				IncludeTags: outputCfg.IncludeTags,
				ExcludeTags: outputCfg.ExcludeTags,
			}
			if outputCfg.Controllers != nil {
				filterOpts.ControllerInclude = outputCfg.Controllers.Include
				filterOpts.ControllerExclude = outputCfg.Controllers.Exclude
			}
			filtered = openapi.FilterControllers(controllers, filterOpts)
		}
		if len(filtered) == 0 {
			label := outputCfg.Output
			if outputCfg.Name != "" {
				label = outputCfg.Name
			}
			printStatus(os.Stderr, pretty, "·", "skipping %s: no controllers after filtering", label)
			continue
		}

		if err := generateOpenAPIFromOutput(filtered, controllerRegistry, outputCfg, cfg, configDir); err != nil {
			label := outputCfg.Output
			if outputCfg.Name != "" {
				label = outputCfg.Name + " (" + outputCfg.Output + ")"
			}
			fmt.Fprintf(os.Stderr, "error generating OpenAPI %s: %v\n", label, err)
			return 1
		}
	}

	// Print warnings
	var allWarnings []string
	for _, w := range ca.Warnings() {
		allWarnings = append(allWarnings, w.Message)
	}
	allWarnings = append(allWarnings, sharedWalker.Warnings()...)
	if len(allWarnings) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, w := range allWarnings {
			printStatus(os.Stderr, pretty, "●", "%s", w)
		}
	}

	printStatus(os.Stderr, pretty, "✓", "done in %s", time.Since(start).Round(time.Millisecond))
	return 0
}
