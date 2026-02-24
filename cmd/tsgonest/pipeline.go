package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/microsoft/typescript-go/shim/core"
	"github.com/tsgonest/tsgonest/internal/config"
	"github.com/tsgonest/tsgonest/internal/pathalias"
)

// TimingReport collects timing data for each build pipeline phase.
// Using a struct avoids the error-prone 11-parameter function signature.
type TimingReport struct {
	TSConfig    time.Duration
	Program     time.Duration
	Diagnostics time.Duration
	Emit        time.Duration
	Aliases     time.Duration
	Checker     time.Duration
	Companions  time.Duration
	Controllers time.Duration
	OpenAPI     time.Duration
	Total       time.Duration
}

// Print outputs the build timing breakdown to stderr.
func (t *TimingReport) Print() {
	fmt.Fprintf(os.Stderr, "\n--- timing ---\n")
	fmt.Fprintf(os.Stderr, "  tsconfig:      %s\n", t.TSConfig.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  program:       %s\n", t.Program.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  diagnostics:   %s\n", t.Diagnostics.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  emit:          %s\n", t.Emit.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  aliases:       %s\n", t.Aliases.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  checker:       %s\n", t.Checker.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  companions:    %s\n", t.Companions.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  controllers:   %s\n", t.Controllers.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  openapi:       %s\n", t.OpenAPI.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "  total:         %s\n", t.Total.Round(time.Millisecond))
}

// ConfigResult holds the result of loading a tsgonest config file.
type ConfigResult struct {
	Config *config.Config
	Path   string // resolved absolute path to config file (empty if none found)
	Dir    string // directory containing the config file (defaults to cwd)
}

// loadOrDiscoverConfig loads a tsgonest config from the given path,
// or auto-discovers one in the working directory if configPath is empty.
// This is shared across build, dev, and sdk commands.
func loadOrDiscoverConfig(configPath, cwd string) (*ConfigResult, error) {
	result := &ConfigResult{Dir: cwd}

	if configPath != "" {
		resolved := configPath
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(cwd, resolved)
		}
		cfg, err := config.Load(resolved)
		if err != nil {
			return nil, err
		}
		result.Config = cfg
		result.Path = resolved
		result.Dir = filepath.Dir(resolved)
		return result, nil
	}

	// Auto-discover: tsgonest.config.ts > tsgonest.config.json
	if p := config.Discover(cwd); p != "" {
		cfg, err := config.Load(p)
		if err != nil {
			return nil, err
		}
		result.Config = cfg
		result.Path = p
		result.Dir = filepath.Dir(p)
		return result, nil
	}

	// No config found — not an error
	return result, nil
}

// resolvePathAliases resolves tsconfig path aliases in the emitted JS files.
// Returns the duration spent and any error.
func resolvePathAliases(opts *core.CompilerOptions, cwd string, emittedFiles []string) (time.Duration, error) {
	start := time.Now()

	if opts.Paths == nil || opts.Paths.Size() == 0 {
		return 0, nil
	}

	pathsMap := make(map[string][]string)
	for k, v := range opts.Paths.Entries() {
		pathsMap[k] = v
	}
	pathsBaseDir := opts.GetPathsBasePath(cwd)

	resolver := pathalias.NewPathResolver(pathalias.Config{
		PathsBaseDir: pathsBaseDir,
		OutDir:       opts.OutDir,
		RootDir:      opts.RootDir,
		Paths:        pathsMap,
	})
	count, err := resolver.ResolveAllEmittedFiles(emittedFiles)
	dur := time.Since(start)

	if err != nil {
		return dur, fmt.Errorf("path alias resolution: %w", err)
	}
	if count > 0 {
		fmt.Fprintf(os.Stderr, "resolved path aliases in %d file(s)\n", count)
	}

	return dur, nil
}
