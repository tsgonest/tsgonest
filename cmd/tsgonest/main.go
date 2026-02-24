package main

import (
	"fmt"
	"os"
	"strings"
)

var version = "0.0.1-dev"

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		// No subcommand — default to build (backward compatible)
		return runBuild(os.Args[1:])
	}

	switch os.Args[1] {
	case "build":
		return runBuild(os.Args[2:])
	case "dev":
		return runDev(os.Args[2:])
	case "migrate":
		return runMigrate(os.Args[2:])
	case "sdk":
		return runSDK(os.Args[2:])
	case "--version", "-v":
		fmt.Println("tsgonest", version)
		return 0
	case "--help", "-h":
		printUsage()
		return 0
	default:
		// Check if first arg starts with - (it's a flag, not a subcommand)
		if strings.HasPrefix(os.Args[1], "-") {
			return runBuild(os.Args[1:])
		}
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		return 1
	}
}

func printUsage() {
	fmt.Println("tsgonest - TypeScript compiler with runtime validation, serialization, and OpenAPI generation")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  tsgonest [flags]              Build project (default)")
	fmt.Println("  tsgonest build [flags]        Build project")
	fmt.Println("  tsgonest dev [flags]          Watch mode (build + start + reload)")
	fmt.Println("  tsgonest migrate [flags]      Migrate from class-validator/Nestia to tsgonest")
	fmt.Println("  tsgonest sdk [flags]          Generate TypeScript SDK from OpenAPI spec")
	fmt.Println()
	fmt.Println("Global Flags:")
	fmt.Println("  --version, -v          Print version and exit")
	fmt.Println("  --help, -h             Print this help message")
	fmt.Println()
	fmt.Println("Build Flags:")
	fmt.Println("  --project, -p <path>   Path to tsconfig.json (default: tsconfig.json)")
	fmt.Println("  --config <path>        Path to tsgonest config file (.ts or .json)")
	fmt.Println("  --dump-metadata        Dump type metadata as JSON to stdout (debug)")
	fmt.Println("  --clean                Clean output directory before building")
	fmt.Println("  --assets <glob>        Glob pattern for static assets to copy to output")
	fmt.Println("  --no-check             Skip type checking (syntax errors still reported)")
	fmt.Println("  [tsgo flags]           Any tsgo compiler flag (--strict, --noEmit, etc.)")
	fmt.Println()
	fmt.Println("Migrate Flags:")
	fmt.Println("  --apply                Write changes to disk (default: dry-run preview)")
	fmt.Println("  --include <glob>       Glob pattern for files to migrate (repeatable)")
	fmt.Println("  --tsconfig <path>      Path to tsconfig.json (default: tsconfig.json)")
	fmt.Println("  --cwd <path>           Working directory (default: current directory)")
	fmt.Println("  --force                Run even if git working directory is dirty")
	fmt.Println("  --yes, -y              Accept all prompts (non-interactive)")
	fmt.Println()
	fmt.Println("SDK Flags:")
	fmt.Println("  --input <path>         Path to OpenAPI JSON file (required)")
	fmt.Println("  --output <path>        Output directory for generated SDK (default: ./sdk)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  tsgonest")
	fmt.Println("  tsgonest build")
	fmt.Println("  tsgonest build --project tsconfig.build.json")
	fmt.Println("  tsgonest build --clean --assets '**/*.json'")
	fmt.Println("  tsgonest build --strict --noEmit           # Pass tsgo flags through")
	fmt.Println("  tsgonest --config tsgonest.config.ts --project tsconfig.json")
	fmt.Println("  tsgonest migrate                           # Preview changes (dry-run)")
	fmt.Println("  tsgonest migrate --apply                   # Apply changes (interactive)")
	fmt.Println("  tsgonest migrate --apply --yes             # Apply all changes (non-interactive)")
	fmt.Println("  tsgonest migrate --include 'src/**/*.ts'   # Scan all TS files")
	fmt.Println("  tsgonest migrate --force                   # Skip git dirty check")
	fmt.Println("  tsgonest sdk --input dist/openapi.json     # Generate SDK")
	fmt.Println("  tsgonest sdk --input api.json --output ./client  # Custom output dir")
	fmt.Println()
}
