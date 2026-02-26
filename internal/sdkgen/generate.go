package sdkgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Generate parses an OpenAPI spec and generates a TypeScript SDK.
func Generate(inputPath, outputDir string) error {
	doc, err := ParseOpenAPI(inputPath)
	if err != nil {
		return fmt.Errorf("parsing: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Rename schema names that collide with TypeScript built-in types.
	// e.g., "Record" → "Record_" to avoid shadowing Record<K,V>.
	renameBuiltinCollisions(doc)

	// Remove stale controller directories that are no longer in the spec.
	cleanStaleControllerDirs(outputDir, doc)

	// Generate shared types
	typesCode := generateSharedTypes(doc.Schemas)
	if err := writeFile(filepath.Join(outputDir, "types.ts"), typesCode); err != nil {
		return err
	}

	// Generate client infrastructure (split for tree-shaking)
	clientCode := generateClient()
	if err := writeFile(filepath.Join(outputDir, "client.ts"), clientCode); err != nil {
		return err
	}

	sseCode := generateSSE()
	if err := writeFile(filepath.Join(outputDir, "sse.ts"), sseCode); err != nil {
		return err
	}

	formDataCode := generateFormData()
	if err := writeFile(filepath.Join(outputDir, "form-data.ts"), formDataCode); err != nil {
		return err
	}

	// Generate per-controller files (respecting version grouping)
	for _, ver := range doc.Versions {
		for _, ctrl := range ver.Controllers {
			ctrlDir := controllerOutputDir(outputDir, ver.Version, ctrl.Name)
			if err := os.MkdirAll(ctrlDir, 0o755); err != nil {
				return fmt.Errorf("creating controller directory: %w", err)
			}

			ctrlCode := generateController(ctrl, doc, ver.Version)
			if err := writeFile(filepath.Join(ctrlDir, "index.ts"), ctrlCode); err != nil {
				return err
			}
		}
	}

	// Generate index.ts
	indexCode := generateIndex(doc.Versions)
	if err := writeFile(filepath.Join(outputDir, "index.ts"), indexCode); err != nil {
		return err
	}

	// Auto-format generated output if a formatter is configured in the project
	formatOutput(outputDir)

	return nil
}

// controllerOutputDir returns the output directory for a controller.
// Versioned: sdk/v1/orders/
// Unversioned: sdk/orders/
func controllerOutputDir(baseDir, version, controllerName string) string {
	dirName := controllerDirName(controllerName)
	if version != "" {
		return filepath.Join(baseDir, version, dirName)
	}
	return filepath.Join(baseDir, dirName)
}

// nonAlphanumRe matches non-alphanumeric characters.
var nonAlphanumRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// controllerDirName converts a controller name to a kebab-case directory name.
// e.g., "OrdersController" → "orders", "Cart RecoveryController" → "cart-recovery",
// "AdminDashboardController" → "admin-dashboard"
func controllerDirName(name string) string {
	name = strings.TrimSuffix(name, "Controller")
	if name == "" {
		return "default"
	}
	return toKebabCase(name)
}

// toKebabCase converts PascalCase, camelCase, and space-separated strings to kebab-case.
func toKebabCase(s string) string {
	// Insert hyphens before uppercase letters that follow lowercase letters (camelCase boundaries)
	var result strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			prev := rune(s[i-1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) {
				result.WriteByte('-')
			}
		}
		result.WriteRune(r)
	}
	// Replace spaces and non-alphanumeric chars with hyphens
	out := nonAlphanumRe.ReplaceAllString(result.String(), "-")
	out = strings.Trim(out, "-")
	return strings.ToLower(out)
}

// formatterInfo holds the detected formatter and the project root where its config was found.
type formatterInfo struct {
	name    string // "prettier", "biome", "oxfmt"
	rootDir string // directory where the config file lives (for running npx)
}

// formatOutput detects a project formatter and runs it on the output directory.
// Supported: prettier, biome, oxfmt. Errors are non-fatal (warnings only).
// The formatter is invoked from the directory where its config was found,
// so monorepo setups with configs at the repo root work correctly.
func formatOutput(outputDir string) {
	f := detectFormatter(outputDir)
	if f.name == "" {
		return
	}
	var cmd *exec.Cmd
	switch f.name {
	case "prettier":
		cmd = exec.Command("npx", "prettier", "--write", filepath.Join(outputDir, "**/*.ts"))
	case "biome":
		cmd = exec.Command("npx", "@biomejs/biome", "format", "--write", outputDir)
	case "oxfmt":
		cmd = exec.Command("npx", "oxfmt", "--write", outputDir)
	default:
		return
	}
	cmd.Dir = f.rootDir
	cmd.Run()
}

// detectFormatter walks up from outputDir looking for formatter config files.
// Returns the formatter name and the directory where its config was found.
func detectFormatter(outputDir string) formatterInfo {
	prettierConfigs := []string{".prettierrc", ".prettierrc.json", ".prettierrc.js", ".prettierrc.cjs", ".prettierrc.mjs", ".prettierrc.yml", ".prettierrc.yaml", ".prettierrc.toml", "prettier.config.js", "prettier.config.cjs", "prettier.config.mjs"}
	biomeConfigs := []string{"biome.json", "biome.jsonc"}
	oxfmtConfigs := []string{".oxfmtrc", ".oxfmtrc.json", "oxfmt.json"}

	dir := outputDir
	for {
		for _, name := range prettierConfigs {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return formatterInfo{"prettier", dir}
			}
		}
		for _, name := range biomeConfigs {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return formatterInfo{"biome", dir}
			}
		}
		for _, name := range oxfmtConfigs {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return formatterInfo{"oxfmt", dir}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return formatterInfo{}
}

// cleanStaleControllerDirs removes controller directories that no longer
// correspond to any controller in the OpenAPI spec. This prevents stale SDK
// files from lingering after controllers are removed or excluded.
func cleanStaleControllerDirs(outputDir string, doc *SDKDocument) {
	// Collect all controller directory names that will be generated
	activeCtrlDirs := make(map[string]bool)
	for _, ver := range doc.Versions {
		for _, ctrl := range ver.Controllers {
			dir := controllerOutputDir(outputDir, ver.Version, ctrl.Name)
			activeCtrlDirs[dir] = true
		}
	}

	// Known top-level generated files (not controller directories)
	knownFiles := map[string]bool{
		"types.ts":    true,
		"client.ts":   true,
		"sse.ts":      true,
		"form-data.ts": true,
		"index.ts":    true,
	}

	// Scan the output directory for subdirectories
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return // directory may not exist yet
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(outputDir, entry.Name())
		if knownFiles[entry.Name()] {
			continue
		}
		// Check if this directory (or its subdirectories) is active
		if activeCtrlDirs[dirPath] {
			continue
		}
		// For versioned layouts (e.g., sdk/v1/orders/), check subdirectories
		isVersionDir := false
		subEntries, err := os.ReadDir(dirPath)
		if err == nil {
			for _, sub := range subEntries {
				if sub.IsDir() {
					subPath := filepath.Join(dirPath, sub.Name())
					if activeCtrlDirs[subPath] {
						isVersionDir = true
					}
				}
			}
		}
		if isVersionDir {
			// Clean stale subdirectories within version directories
			for _, sub := range subEntries {
				if sub.IsDir() {
					subPath := filepath.Join(dirPath, sub.Name())
					if !activeCtrlDirs[subPath] {
						os.RemoveAll(subPath)
					}
				}
			}
			continue
		}
		// Not an active controller directory — remove it
		if !activeCtrlDirs[dirPath] {
			os.RemoveAll(dirPath)
		}
	}
}

func writeFile(path, content string) error {
	// Write-if-changed: skip writing if the file already has identical content.
	// This avoids triggering downstream file watchers unnecessarily.
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == content {
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
