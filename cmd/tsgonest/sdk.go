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
	output := fs.String("output", "", "Override output directory for generated SDK")
	configPath := fs.String("config", "", "Path to tsgonest config file")
	nameFilter := fs.String("name", "", "Generate SDK for a specific named OpenAPI output")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	cwd, _ := os.Getwd()
	cfgResult, cfgErr := loadOrDiscoverConfig(*configPath, cwd)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", cfgErr)
		return 1
	}
	cfg := cfgResult.Config
	if cfg == nil {
		fmt.Fprintln(os.Stderr, "error: no tsgonest config found (required for SDK generation)")
		fmt.Fprintln(os.Stderr, "usage: tsgonest sdk [--name <output-name>] [--output <dir>]")
		return 1
	}

	// Build shared SDK options from config
	var sdkOpts *sdkgen.GenerateOptions
	sdkOpts = &sdkgen.GenerateOptions{
		GlobalPrefix: cfg.NestJS.GlobalPrefix,
	}
	if cfg.NestJS.Versioning != nil && cfg.NestJS.Versioning.Prefix != "" {
		sdkOpts.VersionPrefix = cfg.NestJS.Versioning.Prefix
	}

	// Collect outputs to generate
	type sdkTarget struct {
		input, output, label string
		tsEnums              bool
	}
	var targets []sdkTarget

	for _, oc := range cfg.OpenAPIOutputs {
		if oc.SDK == nil || oc.SDK.Output == "" || oc.Output == "" {
			continue
		}
		if *nameFilter != "" && oc.Name != *nameFilter {
			continue
		}

		sdkInput := oc.Output
		if !filepath.IsAbs(sdkInput) {
			sdkInput = filepath.Join(cfgResult.Dir, sdkInput)
		}
		sdkOutput := oc.SDK.Output
		if *output != "" && (*nameFilter == "" || oc.Name == *nameFilter) {
			sdkOutput = *output
		}
		if !filepath.IsAbs(sdkOutput) {
			sdkOutput = filepath.Join(cfgResult.Dir, sdkOutput)
		}
		label := oc.Output
		if oc.Name != "" {
			label = oc.Name
		}
		targets = append(targets, sdkTarget{sdkInput, sdkOutput, label, oc.SDK.TSEnums})
	}

	if *nameFilter != "" && len(targets) == 0 {
		fmt.Fprintf(os.Stderr, "error: no OpenAPI output named %q with sdk.output configured\n", *nameFilter)
		return 1
	}
	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "error: no OpenAPI outputs have sdk.output configured")
		fmt.Fprintln(os.Stderr, "hint: add sdk: { output: './sdk' } to your openapi output config")
		return 1
	}

	for _, t := range targets {
		opts := *sdkOpts // copy shared options
		opts.TSEnums = t.tsEnums
		if err := sdkgen.Generate(t.input, t.output, &opts); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("SDK generated at %s (from %s)\n", t.output, t.label)
	}
	return 0
}
