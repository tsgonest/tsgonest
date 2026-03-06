package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/microsoft/typescript-go/shim/core"
	"github.com/tsgonest/tsgonest/internal/compiler"
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
	pretty := compiler.IsPrettyOutput()
	dim, reset := "", ""
	if pretty {
		dim = "\u001b[2m"
		reset = "\u001b[0m"
	}
	fmt.Fprintf(os.Stderr, "\n%s── timing ──────────────────────────%s\n", dim, reset)
	fmt.Fprintf(os.Stderr, "%s  tsconfig      %s%s\n", dim, t.TSConfig.Round(time.Millisecond), reset)
	fmt.Fprintf(os.Stderr, "%s  program       %s%s\n", dim, t.Program.Round(time.Millisecond), reset)
	fmt.Fprintf(os.Stderr, "%s  diagnostics   %s%s\n", dim, t.Diagnostics.Round(time.Millisecond), reset)
	fmt.Fprintf(os.Stderr, "%s  emit          %s%s\n", dim, t.Emit.Round(time.Millisecond), reset)
	fmt.Fprintf(os.Stderr, "%s  aliases       %s%s\n", dim, t.Aliases.Round(time.Millisecond), reset)
	fmt.Fprintf(os.Stderr, "%s  checker       %s%s\n", dim, t.Checker.Round(time.Millisecond), reset)
	fmt.Fprintf(os.Stderr, "%s  companions    %s%s\n", dim, t.Companions.Round(time.Millisecond), reset)
	fmt.Fprintf(os.Stderr, "%s  controllers   %s%s\n", dim, t.Controllers.Round(time.Millisecond), reset)
	fmt.Fprintf(os.Stderr, "%s  openapi       %s%s\n", dim, t.OpenAPI.Round(time.Millisecond), reset)
	fmt.Fprintf(os.Stderr, "%s  total         %s%s\n", dim, t.Total.Round(time.Millisecond), reset)
	fmt.Fprintf(os.Stderr, "%s────────────────────────────────────%s\n", dim, reset)
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
		printStatus(os.Stderr, compiler.IsPrettyOutput(), "✓", "resolved path aliases in %d file(s)", count)
	}

	return dur, nil
}

// printStatus prints a formatted status line with an icon.
// When pretty=true, the icon is colored (green for ✓, red for ✗, cyan for info icons).
func printStatus(w *os.File, pretty bool, icon string, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if !pretty {
		fmt.Fprintf(w, "%s %s\n", icon, msg)
		return
	}
	green := "\u001b[32m"
	red := "\u001b[91m"
	cyan := "\u001b[96m"
	reset := "\u001b[0m"
	var color string
	switch icon {
	case "✓":
		color = green
	case "✗":
		color = red
	default:
		color = cyan
	}
	fmt.Fprintf(w, "%s%s%s %s\n", color, icon, reset, msg)
}

// formatLocation splits a warning location like "Controller.method() (file:line)"
// into the method name (bold white) and file path (blue/underlined).
// If no parens are found, the whole string is returned in methodColor.
func formatLocation(loc, methodColor, bold, pathColor, reset string) string {
	// Location format: "ClassName.method() (path/to/file.ts:42)"
	if open := strings.LastIndex(loc, " ("); open != -1 && strings.HasSuffix(loc, ")") {
		method := loc[:open]
		path := loc[open+2 : len(loc)-1] // strip " (" and ")"
		return fmt.Sprintf("%s%s%s%s %s%s%s", bold, methodColor, method, reset, pathColor, path, reset)
	}
	return fmt.Sprintf("%s%s%s%s", bold, methodColor, loc, reset)
}
