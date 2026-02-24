package rewrite

import (
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
// 1. Removes the tsgonest import/require line
// 2. Adds companion import/require lines at the top
// 3. Replaces each marker call with the corresponding companion function call
func rewriteMarkers(text string, outputFile string, calls []MarkerCall, companionMap map[string]string, moduleFormat string) string {
	if len(calls) == 0 {
		return text
	}

	// Check for sentinel — already rewritten
	if strings.Contains(text, rewriteSentinel) {
		return text
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

	// Detect and remove tsgonest import line, collect companion imports
	tsgonestImportRemoved := false
	for _, line := range lines {
		if !tsgonestImportRemoved && isTsgonestImportLine(line) {
			tsgonestImportRemoved = true
			continue // skip this line
		}
		result = append(result, line)
	}

	// Generate companion imports
	importLines = companionImports(calls, companionMap, outputFile, moduleFormat)

	// Replace marker calls in the remaining text
	joined := strings.Join(result, "\n")
	for funcName, typeNames := range funcTypeLookup {
		occurrenceIndex[funcName] = 0
		joined = replaceMarkerCalls(joined, funcName, typeNames, &occurrenceIndex)
	}

	// Reassemble: sentinel + companion imports + rewritten body
	var parts []string
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

// markerCallPatterns caches compiled regexps for each marker function.
var markerCallPatterns = map[string]*regexp.Regexp{}

func init() {
	for name := range markerFunctions {
		// Match the function name followed by ( but not preceded by an alphanumeric
		// character (to avoid matching e.g., "promise" when looking for "is").
		// The negative lookbehind ensures we match standalone calls.
		// We match: funcName( and replace just the function name part.
		markerCallPatterns[name] = regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\(`)
	}
}

// replaceMarkerCalls replaces occurrences of `funcName(` with `companionFuncName(` in order.
func replaceMarkerCalls(text string, funcName string, typeNames []string, occurrenceIndex *map[string]int) string {
	pattern := markerCallPatterns[funcName]
	if pattern == nil {
		return text
	}

	idx := (*occurrenceIndex)[funcName]
	text = pattern.ReplaceAllStringFunc(text, func(match string) string {
		if idx >= len(typeNames) {
			return match // no more type names, leave as-is
		}
		typeName := typeNames[idx]
		idx++
		return companionFuncName(funcName, typeName) + "("
	})
	(*occurrenceIndex)[funcName] = idx

	return text
}
