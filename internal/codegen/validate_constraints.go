package codegen

import (
	"fmt"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// generateTransforms emits JS statements that transform the value in-place before validation.
func generateTransforms(e *Emitter, accessor string, transforms []string) {
	for _, t := range transforms {
		switch t {
		case "trim":
			e.Block("if (typeof %s === \"string\")", accessor)
			e.Line("%s = %s.trim();", accessor, accessor)
			e.EndBlock()
		case "toLowerCase":
			e.Block("if (typeof %s === \"string\")", accessor)
			e.Line("%s = %s.toLowerCase();", accessor, accessor)
			e.EndBlock()
		case "toUpperCase":
			e.Block("if (typeof %s === \"string\")", accessor)
			e.Line("%s = %s.toUpperCase();", accessor, accessor)
			e.EndBlock()
		}
	}
}

// generateCoercion emits JS code to coerce string inputs to the declared type.
// This runs before type checks so "123" becomes 123 before the typeof check.
func generateCoercion(e *Emitter, accessor string, typeMeta *metadata.Metadata) {
	if typeMeta.Kind != metadata.KindAtomic {
		return
	}
	switch typeMeta.Atomic {
	case "number":
		// string → number via unary +
		e.Block("if (typeof %s === \"string\")", accessor)
		e.Line("const _n = +%s;", accessor)
		e.Block("if (!Number.isNaN(_n))")
		e.Line("%s = _n;", accessor)
		e.EndBlock()
		e.EndBlock()
	case "boolean":
		// "true"/"false"/"1"/"0" → boolean
		e.Block("if (%s === \"true\" || %s === \"1\")", accessor, accessor)
		e.Line("%s = true;", accessor)
		e.EndBlockSuffix(fmt.Sprintf(" else if (%s === \"false\" || %s === \"0\") {", accessor, accessor))
		e.indent++
		e.Line("%s = false;", accessor)
		e.EndBlock()
	}
	// Date coercion is handled at the type check level (KindNative "Date"),
	// not at the constraint level, so it's omitted here.
}

// generateConstraintChecks emits JS validation checks for JSDoc constraints.
// When typeVerified is true, typeof guards on constraint checks are omitted because
// the type has already been verified by a preceding type check.
func generateConstraintChecks(e *Emitter, accessor string, path string, prop *metadata.Property) {
	generateConstraintChecksInner(e, accessor, path, prop, false)
}

func generateConstraintChecksVerified(e *Emitter, accessor string, path string, prop *metadata.Property) {
	generateConstraintChecksInner(e, accessor, path, prop, true)
}

func generateConstraintChecksInner(e *Emitter, accessor string, path string, prop *metadata.Property, typeVerified bool) {
	c := prop.Constraints
	if c == nil {
		return
	}

	// Emit transforms BEFORE validation checks
	if len(c.Transforms) > 0 {
		generateTransforms(e, accessor, c.Transforms)
	}

	// Emit coercion BEFORE type checks
	if c.Coerce != nil && *c.Coerce {
		generateCoercion(e, accessor, &prop.Type)
	}

	// Helper: use per-constraint error if present, then global ErrorMessage, then default.
	// constraintKey is the Constraints field name (e.g., "format", "minLength", "minimum").
	errMsg := func(constraintKey string, defaultExpected string) string {
		if c.Errors != nil {
			if msg, ok := c.Errors[constraintKey]; ok {
				return jsStringEscape(msg)
			}
		}
		if c.ErrorMessage != nil {
			return jsStringEscape(*c.ErrorMessage)
		}
		return defaultExpected
	}

	// Numeric constraints
	if c.Minimum != nil {
		if typeVerified {
			e.Block("if (%s < %v)", accessor, *c.Minimum)
		} else {
			e.Block("if (typeof %s === \"number\" && %s < %v)", accessor, accessor, *c.Minimum)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"\" + %s });", path, errMsg("minimum", fmt.Sprintf("minimum %v", *c.Minimum)), accessor)
		e.EndBlock()
	}
	if c.Maximum != nil {
		if typeVerified {
			e.Block("if (%s > %v)", accessor, *c.Maximum)
		} else {
			e.Block("if (typeof %s === \"number\" && %s > %v)", accessor, accessor, *c.Maximum)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"\" + %s });", path, errMsg("maximum", fmt.Sprintf("maximum %v", *c.Maximum)), accessor)
		e.EndBlock()
	}
	if c.ExclusiveMinimum != nil {
		if typeVerified {
			e.Block("if (%s <= %v)", accessor, *c.ExclusiveMinimum)
		} else {
			e.Block("if (typeof %s === \"number\" && %s <= %v)", accessor, accessor, *c.ExclusiveMinimum)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"\" + %s });", path, errMsg("exclusiveMinimum", fmt.Sprintf("exclusiveMinimum %v", *c.ExclusiveMinimum)), accessor)
		e.EndBlock()
	}
	if c.ExclusiveMaximum != nil {
		if typeVerified {
			e.Block("if (%s >= %v)", accessor, *c.ExclusiveMaximum)
		} else {
			e.Block("if (typeof %s === \"number\" && %s >= %v)", accessor, accessor, *c.ExclusiveMaximum)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"\" + %s });", path, errMsg("exclusiveMaximum", fmt.Sprintf("exclusiveMaximum %v", *c.ExclusiveMaximum)), accessor)
		e.EndBlock()
	}
	if c.MultipleOf != nil {
		if typeVerified {
			e.Block("if (%s %% %v !== 0)", accessor, *c.MultipleOf)
		} else {
			e.Block("if (typeof %s === \"number\" && %s %% %v !== 0)", accessor, accessor, *c.MultipleOf)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"\" + %s });", path, errMsg("multipleOf", fmt.Sprintf("multipleOf %v", *c.MultipleOf)), accessor)
		e.EndBlock()
	}
	if c.NumericType != nil {
		generateNumericTypeCheck(e, accessor, path, *c.NumericType, c.ErrorMessage, c.Errors, typeVerified)
	}

	// String length constraints
	if c.MinLength != nil {
		if typeVerified {
			e.Block("if (%s.length < %d)", accessor, *c.MinLength)
		} else {
			e.Block("if (typeof %s === \"string\" && %s.length < %d)", accessor, accessor, *c.MinLength)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"length \" + %s.length });", path, errMsg("minLength", fmt.Sprintf("minLength %d", *c.MinLength)), accessor)
		e.EndBlock()
	}
	if c.MaxLength != nil {
		if typeVerified {
			e.Block("if (%s.length > %d)", accessor, *c.MaxLength)
		} else {
			e.Block("if (typeof %s === \"string\" && %s.length > %d)", accessor, accessor, *c.MaxLength)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"length \" + %s.length });", path, errMsg("maxLength", fmt.Sprintf("maxLength %d", *c.MaxLength)), accessor)
		e.EndBlock()
	}

	// Pattern constraint
	if c.Pattern != nil {
		escapedPattern := escapeForRegexLiteral(*c.Pattern)
		if typeVerified {
			e.Block("if (!/%s/.test(%s))", escapedPattern, accessor)
		} else {
			e.Block("if (typeof %s === \"string\" && !/%s/.test(%s))", accessor, escapedPattern, accessor)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: %s });", path, errMsg("pattern", fmt.Sprintf("pattern %s", *c.Pattern)), accessor)
		e.EndBlock()
	}

	// Format constraint
	if c.Format != nil {
		generateFormatCheck(e, accessor, path, *c.Format, c.ErrorMessage, c.Errors, typeVerified)
	}

	// Array constraints
	if c.MinItems != nil {
		if typeVerified {
			e.Block("if (%s.length < %d)", accessor, *c.MinItems)
		} else {
			e.Block("if (Array.isArray(%s) && %s.length < %d)", accessor, accessor, *c.MinItems)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"length \" + %s.length });", path, errMsg("minItems", fmt.Sprintf("minItems %d", *c.MinItems)), accessor)
		e.EndBlock()
	}
	if c.MaxItems != nil {
		if typeVerified {
			e.Block("if (%s.length > %d)", accessor, *c.MaxItems)
		} else {
			e.Block("if (Array.isArray(%s) && %s.length > %d)", accessor, accessor, *c.MaxItems)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"length \" + %s.length });", path, errMsg("maxItems", fmt.Sprintf("maxItems %d", *c.MaxItems)), accessor)
		e.EndBlock()
	}
	if c.UniqueItems != nil && *c.UniqueItems {
		if typeVerified {
			e.Block("if (new Set(%s).size !== %s.length)", accessor, accessor)
		} else {
			e.Block("if (Array.isArray(%s) && new Set(%s).size !== %s.length)", accessor, accessor, accessor)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"duplicate items\" });", path, errMsg("uniqueItems", "uniqueItems"))
		e.EndBlock()
	}

	// String content checks (Zod parity)
	if c.StartsWith != nil {
		escaped := jsStringEscape(*c.StartsWith)
		if typeVerified {
			e.Block("if (!%s.startsWith(\"%s\"))", accessor, escaped)
		} else {
			e.Block("if (typeof %s === \"string\" && !%s.startsWith(\"%s\"))", accessor, accessor, escaped)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: %s });", path, errMsg("startsWith", fmt.Sprintf("startsWith %s", escaped)), accessor)
		e.EndBlock()
	}
	if c.EndsWith != nil {
		escaped := jsStringEscape(*c.EndsWith)
		if typeVerified {
			e.Block("if (!%s.endsWith(\"%s\"))", accessor, escaped)
		} else {
			e.Block("if (typeof %s === \"string\" && !%s.endsWith(\"%s\"))", accessor, accessor, escaped)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: %s });", path, errMsg("endsWith", fmt.Sprintf("endsWith %s", escaped)), accessor)
		e.EndBlock()
	}
	if c.Includes != nil {
		escaped := jsStringEscape(*c.Includes)
		if typeVerified {
			e.Block("if (!%s.includes(\"%s\"))", accessor, escaped)
		} else {
			e.Block("if (typeof %s === \"string\" && !%s.includes(\"%s\"))", accessor, accessor, escaped)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: %s });", path, errMsg("includes", fmt.Sprintf("includes %s", escaped)), accessor)
		e.EndBlock()
	}
	if c.Uppercase != nil && *c.Uppercase {
		if typeVerified {
			e.Block("if (%s !== %s.toUpperCase())", accessor, accessor)
		} else {
			e.Block("if (typeof %s === \"string\" && %s !== %s.toUpperCase())", accessor, accessor, accessor)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: %s });", path, errMsg("uppercase", "uppercase"), accessor)
		e.EndBlock()
	}
	if c.Lowercase != nil && *c.Lowercase {
		if typeVerified {
			e.Block("if (%s !== %s.toLowerCase())", accessor, accessor)
		} else {
			e.Block("if (typeof %s === \"string\" && %s !== %s.toLowerCase())", accessor, accessor, accessor)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: %s });", path, errMsg("lowercase", "lowercase"), accessor)
		e.EndBlock()
	}

	// Custom validator function: Validate<typeof fn>
	if c.ValidateFn != nil {
		fnName := *c.ValidateFn
		e.Block("if (!%s(%s))", fnName, accessor)
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"\" + %s });", path, errMsg("validate", fmt.Sprintf("validate(%s)", fnName)), accessor)
		e.EndBlock()
	}
}

// generateNumericTypeCheck emits validation for @type int32/uint32/int64/uint64/float/double.
// Checks perConstraintErrors["type"] first, then customError (global), then default.
func generateNumericTypeCheck(e *Emitter, accessor string, path string, numType string, customError *string, perConstraintErrors map[string]string, typeVerified bool) {
	errMsg := func(defaultExpected string) string {
		if perConstraintErrors != nil {
			if msg, ok := perConstraintErrors["type"]; ok {
				return jsStringEscape(msg)
			}
		}
		if customError != nil {
			return jsStringEscape(*customError)
		}
		return defaultExpected
	}
	switch numType {
	case "int32":
		if typeVerified {
			e.Block("if (!Number.isInteger(%s) || %s < -2147483648 || %s > 2147483647)", accessor, accessor, accessor)
		} else {
			e.Block("if (typeof %s === \"number\" && (!Number.isInteger(%s) || %s < -2147483648 || %s > 2147483647))", accessor, accessor, accessor, accessor)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"\" + %s });", path, errMsg("int32"), accessor)
		e.EndBlock()
	case "uint32":
		if typeVerified {
			e.Block("if (!Number.isInteger(%s) || %s < 0 || %s > 4294967295)", accessor, accessor, accessor)
		} else {
			e.Block("if (typeof %s === \"number\" && (!Number.isInteger(%s) || %s < 0 || %s > 4294967295))", accessor, accessor, accessor, accessor)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"\" + %s });", path, errMsg("uint32"), accessor)
		e.EndBlock()
	case "int64":
		if typeVerified {
			e.Block("if (!Number.isInteger(%s) || %s < -9007199254740991 || %s > 9007199254740991)", accessor, accessor, accessor)
		} else {
			e.Block("if (typeof %s === \"number\" && (!Number.isInteger(%s) || %s < -9007199254740991 || %s > 9007199254740991))", accessor, accessor, accessor, accessor)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"\" + %s });", path, errMsg("int64"), accessor)
		e.EndBlock()
	case "uint64":
		if typeVerified {
			e.Block("if (!Number.isInteger(%s) || %s < 0 || %s > 9007199254740991)", accessor, accessor, accessor)
		} else {
			e.Block("if (typeof %s === \"number\" && (!Number.isInteger(%s) || %s < 0 || %s > 9007199254740991))", accessor, accessor, accessor, accessor)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"\" + %s });", path, errMsg("uint64"), accessor)
		e.EndBlock()
	case "float":
		if typeVerified {
			e.Block("if (!Number.isFinite(%s))", accessor)
		} else {
			e.Block("if (typeof %s === \"number\" && !Number.isFinite(%s))", accessor, accessor)
		}
		e.Line("errors.push({ path: %q, expected: \"%s\", received: \"\" + %s });", path, errMsg("float"), accessor)
		e.EndBlock()
	case "double":
		// double always passes — no extra check needed (any finite number is valid)
	}
}

// generateFormatCheck emits validation code for a string format constraint.
// Checks perConstraintErrors["format"] first, then customError (global), then default.
func generateFormatCheck(e *Emitter, accessor string, path string, format string, customError *string, perConstraintErrors map[string]string, typeVerified bool) {
	errMsg := func(defaultExpected string) string {
		if perConstraintErrors != nil {
			if msg, ok := perConstraintErrors["format"]; ok {
				return jsStringEscape(msg)
			}
		}
		if customError != nil {
			return jsStringEscape(*customError)
		}
		return defaultExpected
	}

	switch format {
	case "password":
		// No validation — any string passes
		return
	case "regex":
		// Use try/catch to validate regex
		if typeVerified {
			e.Line("try { new RegExp(%s); } catch (_e) { errors.push({ path: %q, expected: \"%s\", received: %s }); }", accessor, path, errMsg("format regex"), accessor)
		} else {
			e.Block("if (typeof %s === \"string\")", accessor)
			e.Line("try { new RegExp(%s); } catch (_e) { errors.push({ path: %q, expected: \"%s\", received: %s }); }", accessor, path, errMsg("format regex"), accessor)
			e.EndBlock()
		}
		return
	}

	// All other formats use regex validation
	pattern, ok := formatRegexes[format]
	if !ok || pattern == "" {
		return
	}

	flags := formatFlags[format]
	var regexLiteral string
	if flags != "" {
		regexLiteral = fmt.Sprintf("/%s/%s", pattern, flags)
	} else {
		regexLiteral = fmt.Sprintf("/%s/", pattern)
	}

	if typeVerified {
		e.Block("if (!%s.test(%s))", regexLiteral, accessor)
	} else {
		e.Block("if (typeof %s === \"string\" && !%s.test(%s))", accessor, regexLiteral, accessor)
	}
	e.Line("errors.push({ path: %q, expected: \"%s\", received: %s });", path, errMsg(fmt.Sprintf("format %s", format)), accessor)
	e.EndBlock()
}
