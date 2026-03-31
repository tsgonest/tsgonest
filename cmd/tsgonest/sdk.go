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
	input := fs.String("input", "", "Path to OpenAPI JSON file (legacy; prefer per-output sdk config)")
	output := fs.String("output", "", "Output directory for generated SDK")
	configPath := fs.String("config", "", "Path to tsgonest config file")
	nameFilter := fs.String("name", "", "Generate SDK for a specific named OpenAPI output")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Always load config (needed for SDK options like globalPrefix)
	cwd, _ := os.Getwd()
	cfgResult, cfgErr := loadOrDiscoverConfig(*configPath, cwd)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", cfgErr)
		return 1
	}

	cfg := cfgResult.Config

	// Build SDK generation options from config
	var sdkOpts *sdkgen.GenerateOptions
	if cfg != nil {
		sdkOpts = &sdkgen.GenerateOptions{
			GlobalPrefix: cfg.NestJS.GlobalPrefix,
		}
		if cfg.NestJS.Versioning != nil && cfg.NestJS.Versioning.Prefix != "" {
			sdkOpts.VersionPrefix = cfg.NestJS.Versioning.Prefix
		}
	}

	// If --name is specified, generate SDK from that specific OpenAPI output
	if *nameFilter != "" && cfg != nil {
		for _, oc := range cfg.OpenAPIOutputs {
			if oc.Name != *nameFilter {
				continue
			}
			if oc.SDK == nil || oc.SDK.Output == "" {
				fmt.Fprintf(os.Stderr, "error: OpenAPI output %q has no sdk.output configured\n", *nameFilter)
				return 1
			}
			sdkInput := oc.Output
			if !filepath.IsAbs(sdkInput) {
				sdkInput = filepath.Join(cfgResult.Dir, sdkInput)
			}
			sdkOutput := oc.SDK.Output
			if *output != "" {
				sdkOutput = *output // CLI override
			}
			if !filepath.IsAbs(sdkOutput) {
				sdkOutput = filepath.Join(cfgResult.Dir, sdkOutput)
			}

			if err := sdkgen.Generate(sdkInput, sdkOutput, sdkOpts); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("SDK generated at %s (from %s)\n", sdkOutput, *nameFilter)
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: no OpenAPI output named %q\n", *nameFilter)
		return 1
	}

	// Legacy mode: resolve input/output from flags or top-level config
	resolvedInput := *input
	resolvedOutput := *output

	if cfg != nil {
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

	if resolvedOutput == "" {
		resolvedOutput = "./sdk"
	}

	if resolvedInput == "" {
		fmt.Fprintln(os.Stderr, "error: --input is required (or configure sdk.input / openapi.output in tsgonest config)")
		fmt.Fprintln(os.Stderr, "usage: tsgonest sdk --input <openapi.json> [--output <dir>]")
		fmt.Fprintln(os.Stderr, "       tsgonest sdk --name <output-name>")
		return 1
	}

	if err := sdkgen.Generate(resolvedInput, resolvedOutput, sdkOpts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("SDK generated at %s\n", resolvedOutput)
	return 0
}
