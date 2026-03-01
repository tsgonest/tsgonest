package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tsgonest/tsgonest/internal/sdkgen"
)

func runSDK(args []string) int {
	fs := flag.NewFlagSet("sdk", flag.ContinueOnError)
	input := fs.String("input", "", "Path to OpenAPI JSON file")
	output := fs.String("output", "", "Output directory for generated SDK")
	configPath := fs.String("config", "", "Path to tsgonest config file")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	resolvedInput := *input
	resolvedOutput := *output

	// Always load config (needed for SDK options like globalPrefix)
	cwd, _ := os.Getwd()
	cfgResult, cfgErr := loadOrDiscoverConfig(*configPath, cwd)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", cfgErr)
		return 1
	}

	// If input/output not specified, try loading from config
	if cfgResult.Config != nil {
		cfg := cfgResult.Config
		if resolvedInput == "" {
			if cfg.SDK.Input != "" {
				resolvedInput = cfg.SDK.Input
			} else if cfg.OpenAPI.Output != "" {
				resolvedInput = cfg.OpenAPI.Output
			}
			if resolvedInput != "" && !filepath.IsAbs(resolvedInput) {
				resolvedInput = filepath.Join(cfgResult.Dir, resolvedInput)
			}
		}
		if resolvedOutput == "" && cfg.SDK.Output != "" {
			resolvedOutput = cfg.SDK.Output
			if !filepath.IsAbs(resolvedOutput) {
				resolvedOutput = filepath.Join(cfgResult.Dir, resolvedOutput)
			}
		}
	}

	// Apply defaults
	if resolvedOutput == "" {
		resolvedOutput = "./sdk"
	}

	if resolvedInput == "" {
		fmt.Fprintln(os.Stderr, "error: --input is required (or configure sdk.input / openapi.output in tsgonest config)")
		fmt.Fprintln(os.Stderr, "usage: tsgonest sdk --input <openapi.json> [--output <dir>]")
		return 1
	}

	// Build SDK generation options from config
	var sdkOpts *sdkgen.GenerateOptions
	if cfgResult.Config != nil {
		cfg := cfgResult.Config
		sdkOpts = &sdkgen.GenerateOptions{
			GlobalPrefix: cfg.NestJS.GlobalPrefix,
		}
		if cfg.NestJS.Versioning != nil && cfg.NestJS.Versioning.Prefix != "" {
			sdkOpts.VersionPrefix = cfg.NestJS.Versioning.Prefix
		}
	}

	if err := sdkgen.Generate(resolvedInput, resolvedOutput, sdkOpts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("SDK generated at %s\n", resolvedOutput)
	return 0
}
