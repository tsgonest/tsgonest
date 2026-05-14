package codegen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// MintParamKind tags how a route parameter is sourced from the incoming Event.
type MintParamKind int

const (
	// MintParamBody — @Body() parsed via event.body.json().
	MintParamBody MintParamKind = iota
	// MintParamQuery — @Query() — DTO-shaped whole-object or single named scalar.
	MintParamQuery
	// MintParamPathParam — @Param() — single named path segment read from event.params.
	MintParamPathParam
	// MintParamHeader — @Headers() — either DTO via headers iteration or single named header.
	MintParamHeader
)

// MintParamInfo describes one handler parameter for the Mint register wrapper.
type MintParamInfo struct {
	Kind MintParamKind
	// Name is the decorator argument (e.g. "id" for @Param("id"), header name for @Headers("x-foo")).
	// Empty when the parameter consumes the entire body/query/headers object.
	Name string
	// LocalName is the JS identifier the wrapper assigns to before passing to the handler.
	LocalName string
	// TypeName is the DTO type name when this param uses a named DTO (assertXxx companion exists).
	TypeName string
	// CompanionImport is the relative module specifier for the DTO companion (when TypeName != "").
	CompanionImport string
	// Atomic captures the scalar type ("string", "number", "boolean") for single-named
	// scalar query/param/header params. Empty for DTO-shaped params.
	Atomic string
	// Constraints are inlined as runtime checks for scalar params.
	Constraints *metadata.Constraints
	// Multipart describes the body parsing strategy when the body contains File
	// or FileStream fields. nil for JSON bodies.
	Multipart *MintMultipartBody
}

// MintMultipartBody describes a body that must be parsed from multipart/form-data
// rather than JSON. Built from the body type's metadata when at least one field
// is a File or FileStream.
type MintMultipartBody struct {
	// Streaming is true when any field is a FileStream. The wrapper consumes
	// `event.body.stream()` via the multipart parser shipped in @mintkit/core.
	Streaming bool
	// Fields enumerates the body type's top-level properties.
	Fields []MintMultipartField
}

// MintMultipartFieldKind identifies the shape of a single multipart field.
type MintMultipartFieldKind int

const (
	// MintFieldScalar is a string/number/boolean form field.
	MintFieldScalar MintMultipartFieldKind = iota
	// MintFieldFile is a single buffered File entry.
	MintFieldFile
	// MintFieldFileArray is a repeated File entry under the same name.
	MintFieldFileArray
	// MintFieldFileStream is a single streaming file entry.
	MintFieldFileStream
)

// MintMultipartField describes one property of a multipart body type.
type MintMultipartField struct {
	Name        string
	Kind        MintMultipartFieldKind
	Required    bool
	Atomic      string // "string"|"number"|"boolean" for MintFieldScalar
	Constraints *metadata.Constraints
}

// MintRouteInfo describes a single route to register with a Mint app router.
type MintRouteInfo struct {
	Method     string
	Path       string
	MethodName string
	Params     []MintParamInfo

	// ReturnTypeName is the named return type (DTO). Empty for primitive or void returns.
	ReturnTypeName string
	// ReturnCompanionImport is the relative specifier of the return type's companion.
	// Empty when ReturnTypeName is empty.
	ReturnCompanionImport string
	// ReturnIsArray is true when the route returns T[] (use serialize + array stringify).
	ReturnIsArray bool
	// ReturnAtomic is the primitive return ("string", "number", "boolean").
	// Set only when there is no DTO return type.
	ReturnAtomic string
	// ReturnVoid is true when the handler returns void / undefined → 204.
	ReturnVoid bool
}

// MintRegisterInput describes the data needed to emit a registerXxxController() helper.
type MintRegisterInput struct {
	ControllerName       string
	ControllerImportPath string
	Routes               []MintRouteInfo
	// ResponseSerializer mirrors transforms.responseSerializer: "guard" (default),
	// "safe", or "none". "none" falls back to JSON.stringify for DTO returns.
	ResponseSerializer string
}

// GenerateMintRegister returns ESM JS source for a companion file that exports
// `register{ControllerName}(app)`. The companion imports the controller class
// plus any DTO companions, parses parameters from the Event, invokes the handler,
// and serialises the result back into a Response.
func GenerateMintRegister(input MintRegisterInput) string {
	serializer := input.ResponseSerializer
	if serializer == "" {
		serializer = "guard"
	}

	// Decide which file-level helpers we need.
	needsQueryObj := false
	needsHeadersObj := false
	needsValidationErr := false
	needsMimeHelper := false
	needsStreamingParser := false
	for _, r := range input.Routes {
		for _, p := range r.Params {
			switch p.Kind {
			case MintParamQuery:
				if p.Name == "" {
					needsQueryObj = true
				} else {
					// Scalar query params may throw validation errors during coercion / constraint checks.
					needsValidationErr = true
				}
			case MintParamHeader:
				if p.Name == "" {
					needsHeadersObj = true
				} else {
					needsValidationErr = true
				}
			case MintParamPathParam:
				if p.Name != "" {
					needsValidationErr = true
				}
			case MintParamBody:
				if p.Multipart != nil {
					needsValidationErr = true
					if p.Multipart.Streaming {
						needsStreamingParser = true
					}
					for _, f := range p.Multipart.Fields {
						if f.Constraints != nil && len(f.Constraints.MimeTypes) > 0 {
							needsMimeHelper = true
						}
					}
				}
			}
		}
	}

	e := NewEmitter()
	e.Line("// Auto-generated by tsgonest — do not edit")

	// Imports.
	e.Line("import { %s } from %q;", input.ControllerName, input.ControllerImportPath)
	companionImports := collectCompanionImports(input, serializer)
	for _, imp := range companionImports {
		e.Line("import { %s } from %q;", strings.Join(imp.Names, ", "), imp.Module)
	}
	if needsMimeHelper || needsStreamingParser {
		names := []string{}
		if needsMimeHelper {
			names = append(names, "matchMimeType")
		}
		if needsStreamingParser {
			names = append(names, "parseMultipartStream", "MultipartByteLimitError")
		}
		e.Line("import { %s } from %q;", strings.Join(names, ", "), "@mintkit/core")
	}
	e.Blank()

	// Helpers (emit once per file).
	if needsValidationErr {
		emitValidationErrorHelper(e)
	}
	if needsQueryObj {
		emitQueryObjectHelper(e)
	}
	if needsHeadersObj {
		emitHeadersObjectHelper(e)
	}
	if needsStreamingParser {
		emitMultipartBoundaryHelper(e)
	}

	// Exported register function.
	e.Block("export function %s(app)", mintRegisterFnName(input.ControllerName))
	for _, r := range input.Routes {
		emitRoute(e, input.ControllerName, r, serializer)
	}
	e.EndBlock()
	return e.String()
}

type mintCompanionImport struct {
	Module string
	Names  []string
}

// collectCompanionImports gathers the deduped set of companion imports needed
// across all routes — assertXxx for body/query/header DTO params, plus
// stringifyXxx/serializeXxx for return types.
func collectCompanionImports(input MintRegisterInput, serializer string) []mintCompanionImport {
	type entry struct {
		names map[string]bool
	}
	byModule := make(map[string]*entry)

	add := func(module, name string) {
		if module == "" || name == "" {
			return
		}
		e := byModule[module]
		if e == nil {
			e = &entry{names: make(map[string]bool)}
			byModule[module] = e
		}
		e.names[name] = true
	}

	for _, r := range input.Routes {
		for _, p := range r.Params {
			if p.TypeName != "" && p.CompanionImport != "" {
				add(p.CompanionImport, "assert"+p.TypeName)
			}
		}
		if r.ReturnTypeName != "" && r.ReturnCompanionImport != "" && serializer != "none" {
			if r.ReturnIsArray {
				add(r.ReturnCompanionImport, "serialize"+r.ReturnTypeName)
			} else {
				add(r.ReturnCompanionImport, "stringify"+r.ReturnTypeName)
			}
		}
	}

	modules := make([]string, 0, len(byModule))
	for m := range byModule {
		modules = append(modules, m)
	}
	sort.Strings(modules)

	out := make([]mintCompanionImport, 0, len(modules))
	for _, m := range modules {
		names := make([]string, 0, len(byModule[m].names))
		for n := range byModule[m].names {
			names = append(names, n)
		}
		sort.Strings(names)
		out = append(out, mintCompanionImport{Module: m, Names: names})
	}
	return out
}

func emitValidationErrorHelper(e *Emitter) {
	// __mint_validation_error builds a TsgonestValidationError-compatible throw
	// so Mint's error mapper (soft name-check) converts it to RFC 9457
	// application/problem+json. We don't import @tsgonest/runtime to keep this
	// file dep-free — the name check is all that matters.
	e.Block("function __mint_validation_error(path, expected, received)")
	e.Line(`const err = new Error("Validation failed: " + path + " (expected " + expected + ", received " + received + ")");`)
	e.Line(`err.name = "TsgonestValidationError";`)
	e.Line(`err.errors = [{ path, expected, received }];`)
	e.Line(`err.status = 400;`)
	e.Line(`return err;`)
	e.EndBlock()
	e.Blank()
}

func emitQueryObjectHelper(e *Emitter) {
	// __mint_query_object turns URLSearchParams into a plain object, collapsing
	// repeated keys into arrays so DTOs with `tags: string[]` work without extra
	// schema annotation.
	e.Block("function __mint_query_object(searchParams)")
	e.Line(`const out = {};`)
	e.Block(`for (const [key, value] of searchParams.entries())`)
	e.Block(`if (Object.prototype.hasOwnProperty.call(out, key))`)
	e.Block(`if (Array.isArray(out[key]))`)
	e.Line(`out[key].push(value);`)
	e.EndBlockSuffix(" else {")
	e.indent++
	e.Line(`out[key] = [out[key], value];`)
	e.indent--
	e.Line(`}`)
	e.EndBlockSuffix(" else {")
	e.indent++
	e.Line(`out[key] = value;`)
	e.indent--
	e.Line(`}`)
	e.EndBlock()
	e.Line(`return out;`)
	e.EndBlock()
	e.Blank()
}

func emitHeadersObjectHelper(e *Emitter) {
	e.Block("function __mint_headers_object(headers)")
	e.Line(`const out = {};`)
	e.Block(`for (const [key, value] of headers.entries())`)
	e.Line(`out[key] = value;`)
	e.EndBlock()
	e.Line(`return out;`)
	e.EndBlock()
	e.Blank()
}

func emitMultipartBoundaryHelper(e *Emitter) {
	// Extracts the boundary parameter from a Content-Type header value.
	// Throws a TsgonestValidationError when the boundary is missing or empty.
	e.Block("function __mint_multipart_boundary(contentType)")
	e.Line(`const match = /boundary=("?)([^";\s]+)\1/i.exec(contentType ?? "");`)
	e.Block(`if (!match)`)
	e.Line(`throw __mint_validation_error("body", "multipart/form-data with boundary", "missing boundary");`)
	e.EndBlock()
	e.Line(`return match[2];`)
	e.EndBlock()
	e.Blank()
}

// emitBufferedMultipartBody emits inline code that reads the entire request body
// via event.body.formData(), validates each declared field, and assembles the
// final object bound to `local`. Used when at least one field is a `File` (or
// `File[]`) but none are FileStream.
func emitBufferedMultipartBody(e *Emitter, local string, body *MintMultipartBody) {
	e.Line(`const __form = await event.body.formData();`)
	e.Line(`const %s = {};`, local)
	for _, f := range body.Fields {
		emitBufferedMultipartField(e, local, f)
	}
}

// emitBufferedMultipartField emits parsing + validation for a single multipart
// field within a buffered body. Throws a TsgonestValidationError on any
// constraint violation.
func emitBufferedMultipartField(e *Emitter, local string, f MintMultipartField) {
	path := "body." + f.Name
	switch f.Kind {
	case MintFieldScalar:
		e.Line(`{`)
		e.indent++
		e.Line(`const __raw = __form.get(%q);`, f.Name)
		if f.Required {
			e.Block(`if (__raw === null || __raw === undefined)`)
			e.Line(`throw __mint_validation_error(%q, %q, "undefined");`, path, fieldExpectedType(f))
			e.EndBlock()
		}
		// Coerce to declared atomic type.
		switch f.Atomic {
		case "number":
			e.Block(`if (__raw !== null && __raw !== undefined && __raw !== "")`)
			e.Line(`const __n = +__raw;`)
			e.Block(`if (Number.isNaN(__n))`)
			e.Line(`throw __mint_validation_error(%q, "number", String(__raw));`, path)
			e.EndBlock()
			e.Line(`%s[%q] = __n;`, local, f.Name)
			e.EndBlock()
		case "boolean":
			e.Block(`if (__raw === "true" || __raw === "1")`)
			e.Line(`%s[%q] = true;`, local, f.Name)
			e.EndBlockSuffix(` else if (__raw === "false" || __raw === "0") {`)
			e.indent++
			e.Line(`%s[%q] = false;`, local, f.Name)
			e.indent--
			e.EndBlockSuffix(` else if (__raw !== null && __raw !== undefined) {`)
			e.indent++
			e.Line(`throw __mint_validation_error(%q, "boolean", String(__raw));`, path)
			e.indent--
			e.Line(`}`)
		default:
			// String (or unspecified) — accept any FormDataEntryValue that's a string.
			e.Block(`if (__raw !== null && __raw !== undefined)`)
			e.Block(`if (typeof __raw !== "string")`)
			e.Line(`throw __mint_validation_error(%q, "string", typeof __raw);`, path)
			e.EndBlock()
			emitBufferedMultipartScalarStringConstraints(e, "__raw", path, f.Constraints)
			e.Line(`%s[%q] = __raw;`, local, f.Name)
			e.EndBlock()
		}
		if f.Atomic == "number" || f.Atomic == "boolean" {
			emitBufferedMultipartScalarNumericConstraints(e, fmt.Sprintf("%s[%q]", local, f.Name), path, f.Atomic, f.Constraints)
		}
		e.indent--
		e.Line(`}`)

	case MintFieldFile:
		e.Line(`{`)
		e.indent++
		e.Line(`const __file = __form.get(%q);`, f.Name)
		if f.Required {
			e.Block(`if (__file === null || __file === undefined || typeof __file === "string")`)
			e.Line(`throw __mint_validation_error(%q, "File", typeof __file === "string" ? "string" : "undefined");`, path)
			e.EndBlock()
			emitFileConstraints(e, "__file", path, f.Constraints)
			e.Line(`%s[%q] = __file;`, local, f.Name)
		} else {
			e.Block(`if (__file !== null && __file !== undefined)`)
			e.Block(`if (typeof __file === "string")`)
			e.Line(`throw __mint_validation_error(%q, "File", "string");`, path)
			e.EndBlock()
			emitFileConstraints(e, "__file", path, f.Constraints)
			e.Line(`%s[%q] = __file;`, local, f.Name)
			e.EndBlock()
		}
		e.indent--
		e.Line(`}`)

	case MintFieldFileArray:
		e.Line(`{`)
		e.indent++
		e.Line(`const __files = __form.getAll(%q);`, f.Name)
		if f.Required {
			e.Block(`if (__files.length === 0)`)
			e.Line(`throw __mint_validation_error(%q, "File[]", "empty");`, path)
			e.EndBlock()
		}
		e.Line(`const __out = [];`)
		e.Block(`for (let __i = 0; __i < __files.length; __i++)`)
		e.Line(`const __file = __files[__i];`)
		e.Block(`if (typeof __file === "string")`)
		e.Line(`throw __mint_validation_error(%q + "[" + __i + "]", "File", "string");`, path)
		e.EndBlock()
		emitFileConstraints(e, "__file", path+"[]", f.Constraints)
		e.Line(`__out.push(__file);`)
		e.EndBlock()
		e.Line(`%s[%q] = __out;`, local, f.Name)
		e.indent--
		e.Line(`}`)
	}
}

func fieldExpectedType(f MintMultipartField) string {
	switch f.Atomic {
	case "number":
		return "number"
	case "boolean":
		return "boolean"
	default:
		return "string"
	}
}

// emitBufferedMultipartScalarStringConstraints emits inline checks for string
// constraints (pattern, minLength, maxLength). Mirrors emitScalarConstraints but
// without the path-specific framing.
func emitBufferedMultipartScalarStringConstraints(e *Emitter, varName, path string, c *metadata.Constraints) {
	if c == nil {
		return
	}
	if c.Pattern != nil {
		raw := *c.Pattern
		e.Block("if (!/%s/.test(%s))", escapeForRegexLiteral(raw), varName)
		e.Line(`throw __mint_validation_error(%q, "pattern %s", %s);`, path, jsStringEscape(raw), varName)
		e.EndBlock()
	}
	if c.MinLength != nil {
		n := *c.MinLength
		e.Block("if (%s.length < %d)", varName, n)
		e.Line(`throw __mint_validation_error(%q, "minLength %d", "length " + %s.length);`, path, n, varName)
		e.EndBlock()
	}
	if c.MaxLength != nil {
		n := *c.MaxLength
		e.Block("if (%s.length > %d)", varName, n)
		e.Line(`throw __mint_validation_error(%q, "maxLength %d", "length " + %s.length);`, path, n, varName)
		e.EndBlock()
	}
}

func emitBufferedMultipartScalarNumericConstraints(e *Emitter, expr, path, atomic string, c *metadata.Constraints) {
	if c == nil || atomic != "number" {
		return
	}
	if c.Minimum != nil {
		v := formatMintNumber(*c.Minimum)
		e.Block("if (%s !== undefined && %s < %s)", expr, expr, v)
		e.Line(`throw __mint_validation_error(%q, "minimum %s", String(%s));`, path, v, expr)
		e.EndBlock()
	}
	if c.Maximum != nil {
		v := formatMintNumber(*c.Maximum)
		e.Block("if (%s !== undefined && %s > %s)", expr, expr, v)
		e.Line(`throw __mint_validation_error(%q, "maximum %s", String(%s));`, path, v, expr)
		e.EndBlock()
	}
}

// emitFileConstraints emits MaxSize/MinSize/MimeTypes checks on a `File`-shaped
// value. `expr` evaluates to a File at the call site; `path` is the JSON-path
// reported on a validation failure.
func emitFileConstraints(e *Emitter, expr, path string, c *metadata.Constraints) {
	if c == nil {
		return
	}
	if c.MaxSize != nil {
		v := *c.MaxSize
		e.Block("if (%s.size > %d)", expr, v)
		e.Line(`throw __mint_validation_error(%q, "maxSize %d", "size " + %s.size);`, path, v, expr)
		e.EndBlock()
	}
	if c.MinSize != nil {
		v := *c.MinSize
		e.Block("if (%s.size < %d)", expr, v)
		e.Line(`throw __mint_validation_error(%q, "minSize %d", "size " + %s.size);`, path, v, expr)
		e.EndBlock()
	}
	if len(c.MimeTypes) > 0 {
		allowed := mimeTypesLiteral(c.MimeTypes)
		e.Block("if (!matchMimeType(%s.type, %s))", expr, allowed)
		e.Line(`throw __mint_validation_error(%q, "mimeTypes " + %s, %s.type);`, path, allowed, expr)
		e.EndBlock()
	}
}

func mimeTypesLiteral(types []string) string {
	parts := make([]string, len(types))
	for i, t := range types {
		parts[i] = fmt.Sprintf("%q", t)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// emitStreamingMultipartBody emits code that reads the request body as a
// streaming multipart payload via parseMultipartStream from @mintkit/core.
// For each declared field, scalar values are buffered fully; FileStream fields
// receive the part's underlying stream wrapped with a per-part size cap.
//
// Constraint: clients must send scalar form fields BEFORE FileStream parts.
// When we hit a FileStream we expose it to the handler immediately; downstream
// scalars are discarded (the handler has already been called). This is the
// pragmatic single-pass approach — it streams the file without buffering it,
// at the cost of strict ordering. Multiple FileStream fields are not supported
// in v1.
func emitStreamingMultipartBody(e *Emitter, local string, body *MintMultipartBody) {
	e.Line(`const __ct = event.request.headers.get("content-type") ?? "";`)
	e.Line(`const __boundary = __mint_multipart_boundary(__ct);`)
	e.Line(`const %s = {};`, local)
	e.Line(`const __seen = {};`)
	e.Line(`const __iter = parseMultipartStream({ stream: event.body.stream(), boundary: __boundary });`)
	e.Line(`let __streamReady = false;`)
	e.Block(`try`)
	e.Block(`for await (const __part of __iter)`)
	e.Line(`__seen[__part.name] = true;`)
	emitStreamingMultipartDispatch(e, local, body.Fields)
	// If we've handed a stream to the caller already, stop iterating.
	e.Block(`if (__streamReady)`)
	e.Line(`break;`)
	e.EndBlock()
	e.EndBlock()
	e.EndBlockSuffix(" catch (err) {")
	e.indent++
	e.Block(`if (err && err.name === "MultipartByteLimitError")`)
	e.Line(`throw __mint_validation_error(err.path ?? "body", "maxSize " + err.limit, "size > " + err.limit);`)
	e.EndBlock()
	e.Line(`throw err;`)
	e.indent--
	e.Line(`}`)
	// Post-loop required-field checks.
	for _, f := range body.Fields {
		if !f.Required {
			continue
		}
		expected := "FileStream"
		if f.Kind == MintFieldScalar {
			expected = fieldExpectedType(f)
		} else if f.Kind == MintFieldFile {
			expected = "File"
		} else if f.Kind == MintFieldFileArray {
			expected = "File[]"
		}
		e.Block(`if (!__seen[%q])`, f.Name)
		e.Line(`throw __mint_validation_error(%q, %q, "missing");`, "body."+f.Name, expected)
		e.EndBlock()
	}
}

// emitStreamingMultipartDispatch emits the per-part branch logic inside the
// `for await (const __part of __iter)` loop.
func emitStreamingMultipartDispatch(e *Emitter, local string, fields []MintMultipartField) {
	for i, f := range fields {
		// First branch opens with `if (...)`. Subsequent branches arrive
		// already inside an `} else if (...) {` block (set up by the previous
		// iteration's EndBlockSuffix below), so we only restore the indent.
		if i == 0 {
			e.Block(`if (__part.name === %q)`, f.Name)
		}
		path := "body." + f.Name
		switch f.Kind {
		case MintFieldScalar:
			e.Line(`const __chunks = [];`)
			e.Block(`for await (const __c of __part.stream)`)
			e.Line(`__chunks.push(__c);`)
			e.EndBlock()
			e.Line(`let __total = 0;`)
			e.Block(`for (const __c of __chunks)`)
			e.Line(`__total += __c.length;`)
			e.EndBlock()
			e.Line(`const __bytes = new Uint8Array(__total);`)
			e.Line(`let __off = 0;`)
			e.Block(`for (const __c of __chunks)`)
			e.Line(`__bytes.set(__c, __off);`)
			e.Line(`__off += __c.length;`)
			e.EndBlock()
			e.Line(`const __raw = new TextDecoder().decode(__bytes);`)
			switch f.Atomic {
			case "number":
				e.Line(`const __n = +__raw;`)
				e.Block(`if (Number.isNaN(__n))`)
				e.Line(`throw __mint_validation_error(%q, "number", __raw);`, path)
				e.EndBlock()
				e.Line(`%s[%q] = __n;`, local, f.Name)
			case "boolean":
				e.Block(`if (__raw === "true" || __raw === "1")`)
				e.Line(`%s[%q] = true;`, local, f.Name)
				e.EndBlockSuffix(` else if (__raw === "false" || __raw === "0") {`)
				e.indent++
				e.Line(`%s[%q] = false;`, local, f.Name)
				e.indent--
				e.EndBlockSuffix(` else {`)
				e.indent++
				e.Line(`throw __mint_validation_error(%q, "boolean", __raw);`, path)
				e.indent--
				e.Line(`}`)
			default:
				emitBufferedMultipartScalarStringConstraints(e, "__raw", path, f.Constraints)
				e.Line(`%s[%q] = __raw;`, local, f.Name)
			}
		case MintFieldFileStream:
			if f.Constraints != nil && len(f.Constraints.MimeTypes) > 0 {
				allowed := mimeTypesLiteral(f.Constraints.MimeTypes)
				e.Block(`if (!matchMimeType(__part.type ?? "", %s))`, allowed)
				e.Line(`throw __mint_validation_error(%q, "mimeTypes " + %s, __part.type ?? "");`, path, allowed)
				e.EndBlock()
			}
			// Hook in the maxSize limit on the parser before exposing the stream.
			if f.Constraints != nil && f.Constraints.MaxSize != nil {
				e.Line(`__iter.setLimit(%d, %q);`, *f.Constraints.MaxSize, path)
			}
			e.Line(`%s[%q] = { name: __part.filename ?? "", type: __part.type ?? "", stream: __part.stream };`, local, f.Name)
			e.Line(`__streamReady = true;`)
		case MintFieldFile, MintFieldFileArray:
			// File / File[] inside a streaming body fall back to buffering.
			e.Line(`const __chunks = [];`)
			e.Block(`for await (const __c of __part.stream)`)
			e.Line(`__chunks.push(__c);`)
			e.EndBlock()
			e.Line(`const __blob = new File(__chunks, __part.filename ?? "", { type: __part.type ?? "" });`)
			emitFileConstraints(e, "__blob", path, f.Constraints)
			if f.Kind == MintFieldFileArray {
				e.Block(`if (!Array.isArray(%s[%q]))`, local, f.Name)
				e.Line(`%s[%q] = [];`, local, f.Name)
				e.EndBlock()
				e.Line(`%s[%q].push(__blob);`, local, f.Name)
			} else {
				e.Line(`%s[%q] = __blob;`, local, f.Name)
			}
		}
		// Chain to the next field, or close with a default "drain" branch.
		if i+1 < len(fields) {
			next := fields[i+1]
			e.EndBlockSuffix(fmt.Sprintf(" else if (__part.name === %q) {", next.Name))
			e.indent++ // Re-enter the new else-if body for the next iteration.
		} else {
			e.EndBlockSuffix(" else {")
			e.indent++
			e.Block(`for await (const __c of __part.stream)`)
			e.Line(`void __c;`)
			e.EndBlock()
			e.indent--
			e.Line(`}`)
		}
	}
}

func emitRoute(e *Emitter, controllerName string, r MintRouteInfo, serializer string) {
	e.Block("app.router.add(%q, %q, async (event) =>", r.Method, r.Path)
	e.Line("const ctrl = event.resolve(%s);", controllerName)

	if routeNeedsURL(r) {
		e.Line("const url = new URL(event.request.url);")
		e.Line("const searchParams = url.searchParams;")
	}

	args := make([]string, 0, len(r.Params))
	streamingBody := false
	for _, p := range r.Params {
		emitParamExtraction(e, p)
		args = append(args, p.LocalName)
		if p.Kind == MintParamBody && p.Multipart != nil && p.Multipart.Streaming {
			streamingBody = true
		}
	}

	if streamingBody {
		// FileStream reads can throw MultipartByteLimitError mid-stream from
		// inside the handler. Catch it here and translate to a validation
		// error so the route returns 400 problem+json instead of 500.
		e.Block("try")
		e.Line("const result = await ctrl.%s(%s);", r.MethodName, strings.Join(args, ", "))
		e.Line("if (result instanceof Response) return result;")
		emitReturnSerialization(e, r, serializer)
		e.EndBlockSuffix(" catch (err) {")
		e.indent++
		e.Block(`if (err && err.name === "MultipartByteLimitError")`)
		e.Line(`throw __mint_validation_error(err.path ?? "body", "maxSize " + err.limit, "size > " + err.limit);`)
		e.EndBlock()
		e.Line(`throw err;`)
		e.indent--
		e.Line(`}`)
	} else {
		e.Line("const result = await ctrl.%s(%s);", r.MethodName, strings.Join(args, ", "))
		e.Line("if (result instanceof Response) return result;")
		emitReturnSerialization(e, r, serializer)
	}
	e.EndBlockSuffix(");")
}

func routeNeedsURL(r MintRouteInfo) bool {
	for _, p := range r.Params {
		if p.Kind == MintParamQuery {
			return true
		}
	}
	return false
}

func emitParamExtraction(e *Emitter, p MintParamInfo) {
	local := p.LocalName
	if local == "" {
		local = "_p"
	}
	switch p.Kind {
	case MintParamBody:
		if p.Multipart != nil {
			if p.Multipart.Streaming {
				emitStreamingMultipartBody(e, local, p.Multipart)
			} else {
				emitBufferedMultipartBody(e, local, p.Multipart)
			}
		} else if p.TypeName != "" {
			e.Line("const %s = assert%s(await event.body.json());", local, p.TypeName)
		} else {
			e.Line("const %s = await event.body.json();", local)
		}

	case MintParamQuery:
		if p.Name == "" {
			if p.TypeName != "" {
				e.Line("const %s = assert%s(__mint_query_object(searchParams));", local, p.TypeName)
			} else {
				e.Line("const %s = __mint_query_object(searchParams);", local)
			}
		} else {
			e.Line("let %s = searchParams.get(%q);", local, p.Name)
			emitScalarCoercion(e, local, p.Name, p.Atomic, "query")
			emitScalarConstraints(e, local, p.Name, p.Atomic, p.Constraints)
		}

	case MintParamPathParam:
		if p.Name == "" {
			e.Line("const %s = event.params;", local)
		} else {
			e.Line("let %s = event.params[%q];", local, p.Name)
			emitScalarCoercion(e, local, p.Name, p.Atomic, "param")
			emitScalarConstraints(e, local, p.Name, p.Atomic, p.Constraints)
		}

	case MintParamHeader:
		if p.Name == "" {
			if p.TypeName != "" {
				e.Line("const %s = assert%s(__mint_headers_object(event.request.headers));", local, p.TypeName)
			} else {
				e.Line("const %s = __mint_headers_object(event.request.headers);", local)
			}
		} else {
			e.Line("let %s = event.request.headers.get(%q);", local, p.Name)
			emitScalarCoercion(e, local, p.Name, p.Atomic, "header")
			emitScalarConstraints(e, local, p.Name, p.Atomic, p.Constraints)
		}
	}
}

// emitScalarCoercion converts a string-typed value to number/boolean inline.
// It throws a TsgonestValidationError-shaped object so the app error mapper
// catches it via the soft name-check.
func emitScalarCoercion(e *Emitter, paramName, sourceName, atomic, kind string) {
	pathStr := paramPath(sourceName, kind)
	switch atomic {
	case "number":
		e.Block("if (%s === null || %s === \"\")", paramName, paramName)
		e.Line("throw __mint_validation_error(%q, \"number\", typeof %s);", pathStr, paramName)
		e.EndBlock()
		e.Line("%s = +%s;", paramName, paramName)
		e.Block("if (Number.isNaN(%s))", paramName)
		e.Line("throw __mint_validation_error(%q, \"number\", \"NaN\");", pathStr)
		e.EndBlock()
	case "boolean":
		e.Block("if (%s === \"true\" || %s === \"1\")", paramName, paramName)
		e.Line("%s = true;", paramName)
		e.EndBlockSuffix(fmt.Sprintf(" else if (%s === \"false\" || %s === \"0\") {", paramName, paramName))
		e.indent++
		e.Line("%s = false;", paramName)
		e.indent--
		e.EndBlockSuffix(" else {")
		e.indent++
		e.Line("throw __mint_validation_error(%q, \"boolean\", typeof %s);", pathStr, paramName)
		e.indent--
		e.Line("}")
	case "string", "":
		// For required string params, reject null (missing). When kind=="param"
		// the framework guarantees a value (the router only matches if the
		// segment is present), so null indicates an internal contradiction.
		e.Block("if (%s === null)", paramName)
		e.Line("throw __mint_validation_error(%q, \"string\", \"undefined\");", pathStr)
		e.EndBlock()
	}
}

func paramPath(name, kind string) string {
	switch kind {
	case "query":
		return "query." + name
	case "param":
		return "params." + name
	case "header":
		return "headers." + name
	}
	return name
}

func emitScalarConstraints(e *Emitter, paramName, sourceName, atomic string, c *metadata.Constraints) {
	if c == nil {
		return
	}
	pathStr := paramPath(sourceName, "query")
	if atomic == "string" {
		if c.Pattern != nil {
			raw := *c.Pattern
			e.Block("if (!/%s/.test(%s))", escapeForRegexLiteral(raw), paramName)
			e.Line("throw __mint_validation_error(%q, \"pattern %s\", %s);", pathStr, jsStringEscape(raw), paramName)
			e.EndBlock()
		}
		if c.MinLength != nil {
			n := *c.MinLength
			e.Block("if (%s.length < %d)", paramName, n)
			e.Line("throw __mint_validation_error(%q, \"minLength %d\", \"length \" + %s.length);", pathStr, n, paramName)
			e.EndBlock()
		}
		if c.MaxLength != nil {
			n := *c.MaxLength
			e.Block("if (%s.length > %d)", paramName, n)
			e.Line("throw __mint_validation_error(%q, \"maxLength %d\", \"length \" + %s.length);", pathStr, n, paramName)
			e.EndBlock()
		}
	}
	if atomic == "number" {
		if c.Minimum != nil {
			v := formatMintNumber(*c.Minimum)
			e.Block("if (%s < %s)", paramName, v)
			e.Line("throw __mint_validation_error(%q, \"minimum %s\", String(%s));", pathStr, v, paramName)
			e.EndBlock()
		}
		if c.Maximum != nil {
			v := formatMintNumber(*c.Maximum)
			e.Block("if (%s > %s)", paramName, v)
			e.Line("throw __mint_validation_error(%q, \"maximum %s\", String(%s));", pathStr, v, paramName)
			e.EndBlock()
		}
		if c.MultipleOf != nil {
			v := *c.MultipleOf
			if v == float64(int64(v)) {
				vs := formatMintNumber(v)
				e.Block("if (%s %% %s !== 0)", paramName, vs)
				e.Line("throw __mint_validation_error(%q, \"multipleOf %s\", String(%s));", pathStr, vs, paramName)
				e.EndBlock()
			}
		}
	}
}

func formatMintNumber(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}

// emitReturnSerialization builds the final `new Response(...)` from the
// handler's result, honoring event.response.status / event.response.headers.
func emitReturnSerialization(e *Emitter, r MintRouteInfo, serializer string) {
	if r.ReturnVoid {
		// Always 204 for void returns, regardless of result content.
		e.Line(`const headers = new Headers(event.response.headers);`)
		e.Line(`return new Response(null, { status: event.response.status === 200 ? 204 : event.response.status, headers });`)
		return
	}

	e.Line(`const headers = new Headers(event.response.headers);`)
	e.Line(`if (!headers.has("content-type")) headers.set("content-type", "application/json");`)

	switch {
	case r.ReturnTypeName != "" && serializer != "none":
		if r.ReturnIsArray {
			fn := "serialize" + r.ReturnTypeName
			e.Line(`let __body;`)
			e.Block(`if (Array.isArray(result))`)
			e.Line(`let __parts = "";`)
			e.Block(`for (let __i = 0; __i < result.length; __i++)`)
			e.Line(`if (__i > 0) __parts += ",";`)
			e.Line(`__parts += %s(result[__i]);`, fn)
			e.EndBlock()
			e.Line(`__body = "[" + __parts + "]";`)
			e.EndBlockSuffix(" else {")
			e.indent++
			e.Line(`__body = JSON.stringify(result);`)
			e.indent--
			e.Line(`}`)
			e.Line(`return new Response(__body, { status: event.response.status, headers });`)
		} else {
			fn := "stringify" + r.ReturnTypeName
			e.Line(`return new Response(%s(result), { status: event.response.status, headers });`, fn)
		}
	default:
		// Primitive return, unknown return type, or serializer="none" — JSON.stringify everything.
		e.Line(`return new Response(JSON.stringify(result), { status: event.response.status, headers });`)
	}
}

// GenerateMintRegisterTypes returns the corresponding .tsgonest.d.ts content.
func GenerateMintRegisterTypes(controllerName string) string {
	return "export declare function " + mintRegisterFnName(controllerName) + "(app: any): void;\n"
}

// MintRegisterPath returns the companion file path for a Mint register helper.
// e.g. "src/hello.controller.ts" + "HelloController" → "src/hello.controller.HelloController.tsgonest.js"
//
// The companion sits next to the controller's emitted JS, mirroring the
// existing companion path convention.
func MintRegisterPath(sourceFileName string, controllerName string) string {
	return companionPath(sourceFileName, controllerName)
}

// mintRegisterFnName returns the exported function name. It does not strip the
// "Controller" suffix — `HelloController` → `registerHelloController`.
func mintRegisterFnName(controllerName string) string {
	return "register" + strings.TrimSpace(controllerName)
}
