package rewrite

import (
	"fmt"
	"regexp"
	"strings"
)

// rewriteSentinel is inserted into rewritten files to prevent double-rewriting.
const rewriteSentinel = "/* @tsgonest-rewritten */"

// rewriteMarkers replaces marker function calls in emitted JS with companion function calls.
//
// Since type arguments are erased by tsgo, we match by occurrence order — the Nth
// `is(...)` call in JS corresponds to the Nth MarkerCall with FunctionName=="is".
//
// The function also:
//  1. Removes the tsgonest import/require line (kept when any call has no
//     companion — the runtime markers are identity no-ops, so an unrewritten
//     call must keep its import to avoid a ReferenceError)
//  2. Adds companion import/require lines at the top
//  3. Replaces each marker call with the corresponding companion function call
func rewriteMarkers(text string, outputFile string, calls []MarkerCall, companionMap map[string]string, moduleFormat string, warn func(string)) string {
	if len(calls) == 0 {
		return text
	}

	// Check for sentinel — already rewritten
	if strings.Contains(text, rewriteSentinel) {
		return text
	}

	// Calls whose type never produced a companion are left unrewritten.
	missingTypes := make(map[string]bool)
	for _, call := range calls {
		if _, ok := companionMap[call.TypeName]; !ok {
			if !missingTypes[call.TypeName] && warn != nil {
				warn(fmt.Sprintf("no companion generated for type %q — marker call left as a no-op (validation skipped)", call.TypeName))
			}
			missingTypes[call.TypeName] = true
		}
	}

	// Count occurrences of each marker function to build occurrence index
	occurrenceIndex := make(map[string]int) // functionName → next index

	// Build a lookup: functionName → ordered list of type names
	funcTypeLookup := make(map[string][]string)
	for _, call := range calls {
		funcTypeLookup[call.FunctionName] = append(funcTypeLookup[call.FunctionName], call.TypeName)
	}

	lines := strings.Split(text, "\n")
	var result []string
	var importLines []string

	// Detect and remove tsgonest import line, collect companion imports.
	// For CJS emit, capture the namespace binding (e.g. "tsgonest_1") so the
	// interop call form `(0, tsgonest_1.assert)(x)` can be rewritten too.
	tsgonestImportFound := false
	cjsNamespace := ""
	for _, line := range lines {
		if !tsgonestImportFound && isTsgonestImportLine(line) {
			tsgonestImportFound = true
			if m := cjsNamespaceRe.FindStringSubmatch(line); m != nil {
				cjsNamespace = m[1]
			}
			if len(missingTypes) == 0 {
				continue // strip the line — all calls rewrite to companions
			}
		}
		result = append(result, line)
	}

	// Generate companion imports
	importLines = companionImports(calls, companionMap, outputFile, moduleFormat)

	// Replace marker calls in the remaining text
	joined := strings.Join(result, "\n")
	for funcName, typeNames := range funcTypeLookup {
		occurrenceIndex[funcName] = 0
		joined = replaceMarkerCalls(joined, funcName, cjsNamespace, typeNames, missingTypes, &occurrenceIndex)
	}

	// Reassemble: [use strict +] sentinel + companion imports + rewritten body
	var parts []string

	// Preserve "use strict" directive before sentinel and imports
	for _, prefix := range []string{"\"use strict\";\n", "'use strict';\n"} {
		if strings.HasPrefix(joined, prefix) {
			parts = append(parts, strings.TrimSuffix(prefix, "\n"))
			joined = joined[len(prefix):]
			break
		}
	}

	parts = append(parts, rewriteSentinel)
	parts = append(parts, importLines...)
	parts = append(parts, joined)

	return strings.Join(parts, "\n")
}

// isTsgonestImportLine detects ESM or CJS import lines for "tsgonest".
func isTsgonestImportLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	// ESM: import { ... } from "tsgonest"; or from 'tsgonest';
	if strings.HasPrefix(trimmed, "import ") &&
		(strings.Contains(trimmed, `from "tsgonest"`) || strings.Contains(trimmed, `from 'tsgonest'`)) {
		return true
	}
	// CJS: const { ... } = require("tsgonest"); or require('tsgonest');
	if (strings.HasPrefix(trimmed, "const ") || strings.HasPrefix(trimmed, "var ") || strings.HasPrefix(trimmed, "let ")) &&
		(strings.Contains(trimmed, `require("tsgonest")`) || strings.Contains(trimmed, `require('tsgonest')`)) {
		return true
	}
	return false
}

// cjsNamespaceRe extracts the namespace binding from tsgo's CJS emit of the
// tsgonest import, e.g. `const tsgonest_1 = require("tsgonest");` → "tsgonest_1".
// Destructured requires (`const { assert } = ...`) intentionally don't match.
var cjsNamespaceRe = regexp.MustCompile(`(?:const|var|let)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*require\((?:"tsgonest"|'tsgonest')\)`)

// markerCallPattern matches every emitted call form of a marker function:
//   - bare ESM / destructured-require form: `assert(`
//   - CJS interop form: `(0, tsgonest_1.assert)(`
//   - plain member form: `tsgonest_1.assert(`
func markerCallPattern(funcName, cjsNamespace string) *regexp.Regexp {
	name := regexp.QuoteMeta(funcName)
	if cjsNamespace == "" {
		return regexp.MustCompile(`\b` + name + `\(`)
	}
	ns := regexp.QuoteMeta(cjsNamespace)
	return regexp.MustCompile(
		`\(0,\s*` + ns + `\.` + name + `\)\s*\(` +
			`|\b` + ns + `\.` + name + `\(` +
			`|\b` + name + `\(`)
}

// replaceMarkerCalls replaces marker call sites with companion function calls in order.
// Occurrences whose type has no companion are consumed but left unchanged.
func replaceMarkerCalls(text string, funcName string, cjsNamespace string, typeNames []string, missingTypes map[string]bool, occurrenceIndex *map[string]int) string {
	pattern := markerCallPattern(funcName, cjsNamespace)

	idx := (*occurrenceIndex)[funcName]
	text = pattern.ReplaceAllStringFunc(text, func(match string) string {
		if idx >= len(typeNames) {
			return match // no more type names, leave as-is
		}
		typeName := typeNames[idx]
		idx++
		if missingTypes[typeName] {
			return match
		}
		return companionFuncName(funcName, typeName) + "("
	})
	(*occurrenceIndex)[funcName] = idx

	return text
}
