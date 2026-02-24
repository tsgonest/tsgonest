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

	// If input/output not specified, try loading from config
	if resolvedInput == "" || resolvedOutput == "" {
		cwd, _ := os.Getwd()
		cfgResult, cfgErr := loadOrDiscoverConfig(*configPath, cwd)
		if cfgErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", cfgErr)
			return 1
		}

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

	if err := sdkgen.Generate(resolvedInput, resolvedOutput); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("SDK generated at %s\n", resolvedOutput)
	return 0
}
