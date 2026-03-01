package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/typescript-go/shim/core"
	"github.com/tsgonest/tsgonest/internal/compiler"
)

// ── parseBuildArgs tests ─────────────────────────────────────────────────────

func TestParseBuildArgs_Defaults(t *testing.T) {
	f := parseBuildArgs(nil)
	if f.TsconfigPath != "tsconfig.json" {
		t.Errorf("TsconfigPath = %q, want %q", f.TsconfigPath, "tsconfig.json")
	}
	if f.ConfigPath != "" {
		t.Errorf("ConfigPath = %q, want empty", f.ConfigPath)
	}
	if f.DumpMetadata || f.Clean || f.NoCheck {
		t.Error("boolean flags should be false by default")
	}
	if f.Assets != "" {
		t.Errorf("Assets = %q, want empty", f.Assets)
	}
	if len(f.TsgoArgs) != 0 {
		t.Errorf("TsgoArgs = %v, want empty", f.TsgoArgs)
	}
}

func TestParseBuildArgs_TsgonestFlags(t *testing.T) {
	args := []string{
		"--config", "tsgonest.config.ts",
		"--project", "tsconfig.build.json",
		"--dump-metadata",
		"--clean",
		"--assets", "**/*.json",
		"--no-check",
	}
	f := parseBuildArgs(args)

	if f.ConfigPath != "tsgonest.config.ts" {
		t.Errorf("ConfigPath = %q, want %q", f.ConfigPath, "tsgonest.config.ts")
	}
	if f.TsconfigPath != "tsconfig.build.json" {
		t.Errorf("TsconfigPath = %q, want %q", f.TsconfigPath, "tsconfig.build.json")
	}
	if !f.DumpMetadata {
		t.Error("DumpMetadata should be true")
	}
	if !f.Clean {
		t.Error("Clean should be true")
	}
	if f.Assets != "**/*.json" {
		t.Errorf("Assets = %q, want %q", f.Assets, "**/*.json")
	}
	if !f.NoCheck {
		t.Error("NoCheck should be true")
	}
	if len(f.TsgoArgs) != 0 {
		t.Errorf("TsgoArgs = %v, want empty (all flags are tsgonest flags)", f.TsgoArgs)
	}
}

func TestParseBuildArgs_ProjectShortFlag(t *testing.T) {
	f := parseBuildArgs([]string{"-p", "tsconfig.app.json"})
	if f.TsconfigPath != "tsconfig.app.json" {
		t.Errorf("TsconfigPath = %q, want %q", f.TsconfigPath, "tsconfig.app.json")
	}
}

func TestParseBuildArgs_TsgoFlagsPassthrough(t *testing.T) {
	args := []string{"--strict", "--noEmit", "--target", "es2022"}
	f := parseBuildArgs(args)

	if len(f.TsgoArgs) != 4 {
		t.Fatalf("TsgoArgs len = %d, want 4; got %v", len(f.TsgoArgs), f.TsgoArgs)
	}
	expected := []string{"--strict", "--noEmit", "--target", "es2022"}
	for i, want := range expected {
		if f.TsgoArgs[i] != want {
			t.Errorf("TsgoArgs[%d] = %q, want %q", i, f.TsgoArgs[i], want)
		}
	}
}

func TestParseBuildArgs_MixedFlags(t *testing.T) {
	args := []string{
		"--clean",
		"--strict",
		"--project", "tsconfig.build.json",
		"--noEmit",
		"--config", "tsgonest.config.ts",
		"--target", "es2022",
		"--no-check",
		"--sourceMap",
	}
	f := parseBuildArgs(args)

	// Tsgonest flags
	if !f.Clean {
		t.Error("Clean should be true")
	}
	if f.TsconfigPath != "tsconfig.build.json" {
		t.Errorf("TsconfigPath = %q, want %q", f.TsconfigPath, "tsconfig.build.json")
	}
	if f.ConfigPath != "tsgonest.config.ts" {
		t.Errorf("ConfigPath = %q, want %q", f.ConfigPath, "tsgonest.config.ts")
	}
	if !f.NoCheck {
		t.Error("NoCheck should be true")
	}

	// Tsgo flags (order preserved)
	expected := []string{"--strict", "--noEmit", "--target", "es2022", "--sourceMap"}
	if len(f.TsgoArgs) != len(expected) {
		t.Fatalf("TsgoArgs = %v, want %v", f.TsgoArgs, expected)
	}
	for i, want := range expected {
		if f.TsgoArgs[i] != want {
			t.Errorf("TsgoArgs[%d] = %q, want %q", i, f.TsgoArgs[i], want)
		}
	}
}

func TestParseBuildArgs_ValueFlagAtEnd(t *testing.T) {
	// --config without a value at end of args — should not panic
	f := parseBuildArgs([]string{"--config"})
	if f.ConfigPath != "" {
		t.Errorf("ConfigPath = %q, want empty (no value provided)", f.ConfigPath)
	}
}

func TestParseBuildArgs_ProjectFlagAtEnd(t *testing.T) {
	f := parseBuildArgs([]string{"--project"})
	if f.TsconfigPath != "tsconfig.json" {
		t.Errorf("TsconfigPath = %q, want default %q", f.TsconfigPath, "tsconfig.json")
	}
}

func TestParseBuildArgs_AssetsFlagAtEnd(t *testing.T) {
	f := parseBuildArgs([]string{"--assets"})
	if f.Assets != "" {
		t.Errorf("Assets = %q, want empty (no value provided)", f.Assets)
	}
}

func TestParseBuildArgs_EmptyArgs(t *testing.T) {
	f := parseBuildArgs([]string{})
	if f.TsconfigPath != "tsconfig.json" {
		t.Errorf("TsconfigPath = %q, want %q", f.TsconfigPath, "tsconfig.json")
	}
	if len(f.TsgoArgs) != 0 {
		t.Errorf("TsgoArgs = %v, want empty", f.TsgoArgs)
	}
}

func TestParseBuildArgs_OnlyTsgoFlags(t *testing.T) {
	args := []string{"--strict", "--noEmit", "--declaration", "--sourceMap"}
	f := parseBuildArgs(args)

	// No tsgonest flags consumed
	if f.TsconfigPath != "tsconfig.json" {
		t.Errorf("TsconfigPath = %q, want default", f.TsconfigPath)
	}
	if f.Clean || f.DumpMetadata || f.NoCheck {
		t.Error("no boolean tsgonest flags should be set")
	}

	// All forwarded to tsgo
	if len(f.TsgoArgs) != 4 {
		t.Errorf("TsgoArgs len = %d, want 4", len(f.TsgoArgs))
	}
}

func TestParseBuildArgs_RepeatedFlags(t *testing.T) {
	// Last value wins for tsgonest flags
	args := []string{
		"--project", "first.json",
		"--project", "second.json",
	}
	f := parseBuildArgs(args)
	if f.TsconfigPath != "second.json" {
		t.Errorf("TsconfigPath = %q, want %q (last value wins)", f.TsconfigPath, "second.json")
	}
}

// ── parseTsgoFlags tests ────────────────────────────────────────────────────

func TestParseTsgoFlags_Empty(t *testing.T) {
	opts, errs := parseTsgoFlags(nil)
	if opts != nil {
		t.Errorf("expected nil options for empty args, got %+v", opts)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestParseTsgoFlags_Strict(t *testing.T) {
	opts, errs := parseTsgoFlags([]string{"--strict"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if opts.Strict != core.TSTrue {
		t.Errorf("Strict = %v, want TSTrue", opts.Strict)
	}
}

func TestParseTsgoFlags_NoEmit(t *testing.T) {
	opts, errs := parseTsgoFlags([]string{"--noEmit"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if opts.NoEmit != core.TSTrue {
		t.Errorf("NoEmit = %v, want TSTrue", opts.NoEmit)
	}
}

func TestParseTsgoFlags_TargetES2022(t *testing.T) {
	opts, errs := parseTsgoFlags([]string{"--target", "es2022"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if opts.Target != core.ScriptTargetES2022 {
		t.Errorf("Target = %v, want ScriptTargetES2022 (%v)", opts.Target, core.ScriptTargetES2022)
	}
}

func TestParseTsgoFlags_TargetESNext(t *testing.T) {
	opts, errs := parseTsgoFlags([]string{"--target", "esnext"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if opts.Target != core.ScriptTargetESNext {
		t.Errorf("Target = %v, want ScriptTargetESNext (%v)", opts.Target, core.ScriptTargetESNext)
	}
}

func TestParseTsgoFlags_SourceMap(t *testing.T) {
	opts, errs := parseTsgoFlags([]string{"--sourceMap"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if opts.SourceMap != core.TSTrue {
		t.Errorf("SourceMap = %v, want TSTrue", opts.SourceMap)
	}
}

func TestParseTsgoFlags_Declaration(t *testing.T) {
	opts, errs := parseTsgoFlags([]string{"--declaration"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if opts.Declaration != core.TSTrue {
		t.Errorf("Declaration = %v, want TSTrue", opts.Declaration)
	}
}

func TestParseTsgoFlags_MultipleFlags(t *testing.T) {
	opts, errs := parseTsgoFlags([]string{"--strict", "--noEmit", "--target", "es2022", "--sourceMap"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if opts.Strict != core.TSTrue {
		t.Errorf("Strict = %v, want TSTrue", opts.Strict)
	}
	if opts.NoEmit != core.TSTrue {
		t.Errorf("NoEmit = %v, want TSTrue", opts.NoEmit)
	}
	if opts.Target != core.ScriptTargetES2022 {
		t.Errorf("Target = %v, want ScriptTargetES2022", opts.Target)
	}
	if opts.SourceMap != core.TSTrue {
		t.Errorf("SourceMap = %v, want TSTrue", opts.SourceMap)
	}
}

func TestParseTsgoFlags_InvalidFlag(t *testing.T) {
	_, errs := parseTsgoFlags([]string{"--invalidFlagXyz123"})
	if len(errs) == 0 {
		t.Error("expected error for invalid flag, got none")
	}
}

func TestParseTsgoFlags_InvalidTargetValue(t *testing.T) {
	_, errs := parseTsgoFlags([]string{"--target", "es1999"})
	if len(errs) == 0 {
		t.Error("expected error for invalid target value, got none")
	}
}

// ── ParseTSConfig with CLI overrides integration tests ──────────────────────

// setupTSProject creates a temp dir with a tsconfig.json and a dummy source file.
// Returns the directory path. tsgo requires at least one source file.
func setupTSProject(t *testing.T, tsconfigContent string) string {
	t.Helper()
	dir := t.TempDir()

	// Write tsconfig
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfigContent), 0644)

	// Create a dummy source file so tsgo doesn't complain about empty file list
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "src", "index.ts"), []byte("export const x = 1;\n"), 0644)

	return dir
}

func TestParseTSConfig_CLIOverrideStrict(t *testing.T) {
	dir := setupTSProject(t, `{
		"compilerOptions": {
			"target": "es2015",
			"outDir": "./dist",
			"strict": false
		},
		"include": ["src/**/*.ts"]
	}`)

	fs := compiler.CreateDefaultFS()
	host := compiler.CreateDefaultHost(dir, fs)

	// Parse with --strict override
	overrides := &core.CompilerOptions{
		Strict: core.TSTrue,
	}
	parsed, diags, err := compiler.ParseTSConfig(fs, dir, "tsconfig.json", host, overrides)
	if err != nil {
		t.Fatalf("ParseTSConfig error: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("ParseTSConfig diagnostics: %v", diags)
	}

	opts := parsed.CompilerOptions()
	if opts.Strict != core.TSTrue {
		t.Errorf("Strict = %v, want TSTrue (CLI override should win)", opts.Strict)
	}
}

func TestParseTSConfig_CLIOverrideNoEmit(t *testing.T) {
	dir := setupTSProject(t, `{
		"compilerOptions": {
			"target": "es2022",
			"outDir": "./dist"
		},
		"include": ["src/**/*.ts"]
	}`)

	fs := compiler.CreateDefaultFS()
	host := compiler.CreateDefaultHost(dir, fs)

	overrides := &core.CompilerOptions{
		NoEmit: core.TSTrue,
	}
	parsed, diags, err := compiler.ParseTSConfig(fs, dir, "tsconfig.json", host, overrides)
	if err != nil {
		t.Fatalf("ParseTSConfig error: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("ParseTSConfig diagnostics: %v", diags)
	}

	opts := parsed.CompilerOptions()
	if opts.NoEmit != core.TSTrue {
		t.Errorf("NoEmit = %v, want TSTrue (CLI override should win)", opts.NoEmit)
	}
}

func TestParseTSConfig_CLIOverrideTarget(t *testing.T) {
	dir := setupTSProject(t, `{
		"compilerOptions": {
			"target": "es2015",
			"outDir": "./dist"
		},
		"include": ["src/**/*.ts"]
	}`)

	fs := compiler.CreateDefaultFS()
	host := compiler.CreateDefaultHost(dir, fs)

	overrides := &core.CompilerOptions{
		Target: core.ScriptTargetES2022,
	}
	parsed, diags, err := compiler.ParseTSConfig(fs, dir, "tsconfig.json", host, overrides)
	if err != nil {
		t.Fatalf("ParseTSConfig error: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("ParseTSConfig diagnostics: %v", diags)
	}

	opts := parsed.CompilerOptions()
	if opts.Target != core.ScriptTargetES2022 {
		t.Errorf("Target = %v, want ScriptTargetES2022 (%v)", opts.Target, core.ScriptTargetES2022)
	}
}

func TestParseTSConfig_NilOverridesUsesDefaults(t *testing.T) {
	dir := setupTSProject(t, `{
		"compilerOptions": {
			"target": "es2015",
			"strict": true,
			"outDir": "./dist"
		},
		"include": ["src/**/*.ts"]
	}`)

	fs := compiler.CreateDefaultFS()
	host := compiler.CreateDefaultHost(dir, fs)

	// nil overrides — tsconfig values should be used as-is
	parsed, diags, err := compiler.ParseTSConfig(fs, dir, "tsconfig.json", host, nil)
	if err != nil {
		t.Fatalf("ParseTSConfig error: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("ParseTSConfig diagnostics: %v", diags)
	}

	opts := parsed.CompilerOptions()
	if opts.Strict != core.TSTrue {
		t.Errorf("Strict = %v, want TSTrue (from tsconfig)", opts.Strict)
	}
	if opts.Target != core.ScriptTargetES2015 {
		t.Errorf("Target = %v, want ScriptTargetES2015 (%v)", opts.Target, core.ScriptTargetES2015)
	}
}

// ── End-to-end: parseBuildArgs → parseTsgoFlags pipeline ────────────────────

func TestPipeline_MixedFlagsToParsedOptions(t *testing.T) {
	// Simulate: tsgonest build --clean --strict --project tsconfig.build.json --noEmit --target es2022
	args := []string{
		"--clean",
		"--strict",
		"--project", "tsconfig.build.json",
		"--noEmit",
		"--target", "es2022",
	}

	flags := parseBuildArgs(args)

	// Verify tsgonest flags consumed correctly
	if !flags.Clean {
		t.Error("Clean should be true")
	}
	if flags.TsconfigPath != "tsconfig.build.json" {
		t.Errorf("TsconfigPath = %q, want %q", flags.TsconfigPath, "tsconfig.build.json")
	}

	// Verify tsgo flags forwarded
	expectedTsgo := []string{"--strict", "--noEmit", "--target", "es2022"}
	if len(flags.TsgoArgs) != len(expectedTsgo) {
		t.Fatalf("TsgoArgs = %v, want %v", flags.TsgoArgs, expectedTsgo)
	}

	// Parse tsgo flags
	opts, errs := parseTsgoFlags(flags.TsgoArgs)
	if len(errs) > 0 {
		t.Fatalf("parseTsgoFlags errors: %v", errs)
	}
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if opts.Strict != core.TSTrue {
		t.Errorf("Strict = %v, want TSTrue", opts.Strict)
	}
	if opts.NoEmit != core.TSTrue {
		t.Errorf("NoEmit = %v, want TSTrue", opts.NoEmit)
	}
	if opts.Target != core.ScriptTargetES2022 {
		t.Errorf("Target = %v, want ScriptTargetES2022", opts.Target)
	}
}

func TestPipeline_NoTsgoFlags(t *testing.T) {
	// All tsgonest flags — nothing should go to tsgo
	args := []string{"--clean", "--no-check", "--project", "tsconfig.json"}
	flags := parseBuildArgs(args)

	if len(flags.TsgoArgs) != 0 {
		t.Errorf("TsgoArgs = %v, want empty", flags.TsgoArgs)
	}

	opts, errs := parseTsgoFlags(flags.TsgoArgs)
	if opts != nil {
		t.Errorf("expected nil options for empty tsgo args, got %+v", opts)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestPipeline_InvalidTsgoFlagReportsError(t *testing.T) {
	args := []string{"--clean", "--badFlag", "--project", "tsconfig.json"}
	flags := parseBuildArgs(args)

	// --badFlag should be forwarded to tsgo
	if len(flags.TsgoArgs) != 1 || flags.TsgoArgs[0] != "--badFlag" {
		t.Fatalf("TsgoArgs = %v, want [--badFlag]", flags.TsgoArgs)
	}

	_, errs := parseTsgoFlags(flags.TsgoArgs)
	if len(errs) == 0 {
		t.Error("expected error for --badFlag, got none")
	}
}

// --- Fix 3: smartCleanDir tests ---

func TestSmartCleanDir_PreservesTsbuildinfo(t *testing.T) {
	dir := t.TempDir()

	// Create files in the output directory
	os.WriteFile(filepath.Join(dir, "main.js"), []byte("console.log('hello')"), 0644)
	os.WriteFile(filepath.Join(dir, "main.d.ts"), []byte("export {}"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "other.js"), []byte("// other"), 0644)

	// Create the .tsbuildinfo file that should be preserved
	tsbuildInfoPath := filepath.Join(dir, "tsconfig.tsbuildinfo")
	os.WriteFile(tsbuildInfoPath, []byte(`{"version": "5.0"}`), 0644)

	// Run smartCleanDir
	err := smartCleanDir(dir, tsbuildInfoPath)
	if err != nil {
		t.Fatalf("smartCleanDir error: %v", err)
	}

	// .tsbuildinfo should still exist
	if _, err := os.Stat(tsbuildInfoPath); os.IsNotExist(err) {
		t.Error(".tsbuildinfo should be preserved after smartCleanDir")
	}

	// Other files should be removed
	if _, err := os.Stat(filepath.Join(dir, "main.js")); !os.IsNotExist(err) {
		t.Error("main.js should be deleted by smartCleanDir")
	}
	if _, err := os.Stat(filepath.Join(dir, "main.d.ts")); !os.IsNotExist(err) {
		t.Error("main.d.ts should be deleted by smartCleanDir")
	}
	if _, err := os.Stat(filepath.Join(dir, "sub")); !os.IsNotExist(err) {
		t.Error("sub/ directory should be deleted by smartCleanDir")
	}
}

func TestSmartCleanDir_NonexistentDir(t *testing.T) {
	// Should not error on a directory that doesn't exist
	err := smartCleanDir("/tmp/nonexistent-dir-xyz-123", "/tmp/nonexistent.tsbuildinfo")
	if err != nil {
		t.Errorf("smartCleanDir on nonexistent dir should not error: %v", err)
	}
}

func TestSmartCleanDir_DangerousPath(t *testing.T) {
	// Should refuse to clean dangerous paths
	for _, dangerous := range []string{"/", ".", ".."} {
		err := smartCleanDir(dangerous, "tsconfig.tsbuildinfo")
		if err == nil {
			t.Errorf("smartCleanDir(%q) should return error for dangerous path", dangerous)
		}
	}
}

func TestSmartCleanDir_NoTsbuildinfo(t *testing.T) {
	// When there's no .tsbuildinfo to preserve, all files should be removed
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.js"), []byte("// js"), 0644)
	os.WriteFile(filepath.Join(dir, "main.d.ts"), []byte("// dts"), 0644)

	// Reference a tsbuildinfo that doesn't exist — all files should be removed
	err := smartCleanDir(dir, filepath.Join(dir, "nonexistent.tsbuildinfo"))
	if err != nil {
		t.Fatalf("smartCleanDir error: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected empty dir after smartCleanDir, got %d entries", len(entries))
	}
}

func TestPipeline_FullIntegrationWithTSConfig(t *testing.T) {
	dir := setupTSProject(t, `{
		"compilerOptions": {
			"target": "es2015",
			"strict": false,
			"outDir": "./dist"
		},
		"include": ["src/**/*.ts"]
	}`)

	// Simulate: tsgonest build --strict --target es2022 --no-check
	args := []string{
		"--strict",
		"--target", "es2022",
		"--no-check",
	}

	flags := parseBuildArgs(args)
	if !flags.NoCheck {
		t.Error("NoCheck should be true")
	}

	opts, errs := parseTsgoFlags(flags.TsgoArgs)
	if len(errs) > 0 {
		t.Fatalf("parseTsgoFlags errors: %v", errs)
	}

	// Now parse tsconfig with CLI overrides
	fs := compiler.CreateDefaultFS()
	host := compiler.CreateDefaultHost(dir, fs)
	parsed, diags, err := compiler.ParseTSConfig(fs, dir, "tsconfig.json", host, opts)
	if err != nil {
		t.Fatalf("ParseTSConfig error: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("ParseTSConfig diagnostics: %v", diags)
	}

	finalOpts := parsed.CompilerOptions()
	// CLI --strict should override tsconfig strict: false
	if finalOpts.Strict != core.TSTrue {
		t.Errorf("Strict = %v, want TSTrue (CLI override)", finalOpts.Strict)
	}
	// CLI --target es2022 should override tsconfig target: es2015
	if finalOpts.Target != core.ScriptTargetES2022 {
		t.Errorf("Target = %v, want ScriptTargetES2022 (CLI override)", finalOpts.Target)
	}
}
