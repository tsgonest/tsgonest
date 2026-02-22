package analyzer

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// ControllerInfo represents a parsed NestJS controller class.
type ControllerInfo struct {
	// Name is the class name (e.g., "UserController").
	Name string
	// Path is the controller route prefix (e.g., "users" from @Controller("users")).
	Path string
	// Routes contains all the extracted HTTP routes.
	Routes []Route
	// SourceFile is the path of the source file containing this controller.
	SourceFile string
}

// Route represents a single HTTP route extracted from a controller method.
type Route struct {
	// Method is the HTTP method (GET, POST, PUT, DELETE, PATCH, etc.).
	Method string
	// Path is the full route path (controller prefix + method sub-path).
	Path string
	// OperationID is derived from the method name (e.g., "findAll", "create").
	OperationID string
	// Parameters holds the route parameters (@Body, @Query, @Param, @Headers).
	Parameters []RouteParameter
	// ReturnType is the resolved return type metadata (Promise<T> unwrapped to T).
	ReturnType metadata.Metadata
	// StatusCode is the HTTP status code (from @HttpCode or convention).
	StatusCode int
	// Summary is from JSDoc @summary tag or first line of JSDoc.
	Summary string
	// Description is from JSDoc body text.
	Description string
	// Deprecated indicates the route is deprecated (from @deprecated tag).
	Deprecated bool
	// Tags are derived from the controller name or @tag JSDoc.
	Tags []string
	// Security holds security requirements (from @security JSDoc tag).
	Security []SecurityRequirement
	// ErrorResponses holds typed error responses (from @throws JSDoc tags).
	ErrorResponses []ErrorResponse
	// Version is from @Version() decorator (e.g., "1", "2").
	Version string
}

// SecurityRequirement represents an OpenAPI security requirement.
type SecurityRequirement struct {
	// Name is the security scheme name (e.g., "bearer", "oauth2").
	Name string
	// Scopes holds OAuth2 scopes (if applicable).
	Scopes []string
}

// ErrorResponse represents a typed error response from @throws.
type ErrorResponse struct {
	// StatusCode is the HTTP error status code (e.g., 400, 401, 404).
	StatusCode int
	// TypeName is the error type name for schema resolution.
	TypeName string
	// Type is the resolved type metadata for the error response body.
	Type metadata.Metadata
}

// RouteParameter represents a parameter extracted from a controller method.
type RouteParameter struct {
	// Category is the parameter source: "body", "query", "param", "headers".
	Category string
	// Name is the parameter name (e.g., "id" from @Param("id")).
	// For @Body() without argument, Name is empty.
	Name string
	// Type is the resolved type metadata.
	Type metadata.Metadata
	// Required indicates whether the parameter is required.
	Required bool
	// ContentType is the request body content type.
	// Default is "application/json". Other values: "text/plain", "multipart/form-data", "application/x-www-form-urlencoded".
	// Only meaningful for body parameters.
	ContentType string
}

// ControllerAnalyzer extracts NestJS controller information from source files.
type ControllerAnalyzer struct {
	program  *shimcompiler.Program
	checker  *shimchecker.Checker
	walker   *TypeWalker
	registry *metadata.TypeRegistry
	warnings *WarningCollector
}

// NewControllerAnalyzer creates a new controller analyzer.
func NewControllerAnalyzer(program *shimcompiler.Program) (*ControllerAnalyzer, func()) {
	checker, release := shimcompiler.Program_GetTypeChecker(program, context.Background())
	walker := NewTypeWalker(checker)

	return &ControllerAnalyzer{
		program:  program,
		checker:  checker,
		walker:   walker,
		registry: walker.Registry(),
		warnings: NewWarningCollector(),
	}, release
}

// NewControllerAnalyzerWithWalker creates a controller analyzer that reuses an existing
// checker and type walker. This allows sharing type registry state with companion
// generation, so types already walked (DTOs) short-circuit to KindRef immediately.
func NewControllerAnalyzerWithWalker(program *shimcompiler.Program, checker *shimchecker.Checker, walker *TypeWalker) *ControllerAnalyzer {
	return &ControllerAnalyzer{
		program:  program,
		checker:  checker,
		walker:   walker,
		registry: walker.Registry(),
		warnings: NewWarningCollector(),
	}
}

// Warnings returns the warnings collected during analysis.
func (a *ControllerAnalyzer) Warnings() []Warning {
	if a.warnings == nil {
		return nil
	}
	return a.warnings.Warnings
}

// Registry returns the type registry collected during analysis.
func (a *ControllerAnalyzer) Registry() *metadata.TypeRegistry {
	return a.registry
}

// AnalyzeSourceFile extracts all controllers from a source file.
func (a *ControllerAnalyzer) AnalyzeSourceFile(sf *ast.SourceFile) []ControllerInfo {
	var controllers []ControllerInfo

	for _, stmt := range sf.Statements.Nodes {
		if stmt.Kind != ast.KindClassDeclaration {
			continue
		}

		info := a.analyzeClass(stmt, sf.FileName())
		if info != nil {
			controllers = append(controllers, *info)
		}
	}

	return controllers
}

// AnalyzeProgram extracts controllers from all source files matching the include patterns.
func (a *ControllerAnalyzer) AnalyzeProgram(includePatterns []string, excludePatterns []string) []ControllerInfo {
	var allControllers []ControllerInfo

	for _, sf := range a.program.GetSourceFiles() {
		if sf.IsDeclarationFile {
			continue
		}

		// Check if the source file matches include/exclude patterns
		if !MatchesGlob(sf.FileName(), includePatterns, excludePatterns) {
			continue
		}

		controllers := a.AnalyzeSourceFile(sf)
		allControllers = append(allControllers, controllers...)
	}

	return allControllers
}

// analyzeClass attempts to parse a class declaration as a NestJS controller.
// Returns nil if the class is not a controller.
func (a *ControllerAnalyzer) analyzeClass(classNode *ast.Node, sourceFile string) *ControllerInfo {
	classDecl := classNode.AsClassDeclaration()

	// Look for @Controller decorator
	controllerPath := ""
	isController := false
	for _, dec := range classNode.Decorators() {
		info := ParseDecorator(dec)
		if info != nil && IsControllerDecorator(info) {
			isController = true
			controllerPath = GetControllerPath(info)
			break
		}
	}

	if !isController {
		return nil
	}

	className := ""
	if classDecl.Name() != nil {
		className = classDecl.Name().Text()
	}

	// Derive tag from class name
	tag := deriveTag(className)

	// Analyze methods
	var routes []Route
	if classDecl.Members != nil {
		for _, member := range classDecl.Members.Nodes {
			if member.Kind != ast.KindMethodDeclaration {
				continue
			}

			route := a.analyzeMethod(member, controllerPath, tag)
			if route != nil {
				routes = append(routes, *route)
			}
		}
	}

	return &ControllerInfo{
		Name:       className,
		Path:       controllerPath,
		Routes:     routes,
		SourceFile: sourceFile,
	}
}

// analyzeMethod attempts to parse a method declaration as a NestJS route handler.
// Returns nil if the method has no HTTP method decorator.
func (a *ControllerAnalyzer) analyzeMethod(methodNode *ast.Node, controllerPath string, tag string) *Route {
	methodDecl := methodNode.AsMethodDeclaration()

	// Look for HTTP method decorators, @HttpCode, and @Version
	httpMethod := ""
	subPath := ""
	statusCode := 0
	version := ""

	for _, dec := range methodNode.Decorators() {
		info := ParseDecorator(dec)
		if info == nil {
			continue
		}

		switch info.Name {
		case "Get", "Post", "Put", "Delete", "Patch", "Head", "Options":
			httpMethod = httpMethodForDecorator(info.Name)
			if len(info.Args) > 0 {
				subPath = info.Args[0]
			}
		case "HttpCode":
			if info.NumericArg != nil {
				statusCode = int(*info.NumericArg)
			}
		case "Version":
			if len(info.Args) > 0 {
				version = info.Args[0]
			}
		}
	}

	if httpMethod == "" {
		return nil
	}

	// Default status codes by convention
	if statusCode == 0 {
		statusCode = defaultStatusCode(httpMethod)
	}

	// Build full path
	fullPath := CombinePaths(controllerPath, subPath)

	// Get method name for operationId
	operationID := ""
	if methodDecl.Name() != nil {
		operationID = methodDecl.Name().Text()
	}

	// Extract parameters
	var params []RouteParameter
	if methodDecl.Parameters != nil {
		for _, paramNode := range methodDecl.Parameters.Nodes {
			param := a.analyzeParameter(paramNode)
			if param != nil {
				params = append(params, *param)
			}
		}
	}

	// Extract return type
	returnType := a.extractReturnType(methodNode, operationID)

	var tags []string
	if tag != "" {
		tags = []string{tag}
	}

	// Extract JSDoc metadata from the method
	summary, description, deprecated, jsdocTags, security, errorResponses, contentType := extractMethodJSDoc(methodNode)
	if len(jsdocTags) > 0 {
		tags = jsdocTags // Override controller-derived tags
	}

	// Apply content type to body parameters:
	// 1. JSDoc @contentType override takes highest priority
	// 2. Auto-detect: string body → text/plain
	// 3. Default: application/json (empty string means default)
	for i := range params {
		if params[i].Category == "body" {
			if contentType != "" {
				params[i].ContentType = contentType
			} else if params[i].Type.Kind == metadata.KindAtomic && params[i].Type.Atomic == "string" {
				params[i].ContentType = "text/plain"
			}
			// else: leave empty, generator will default to application/json
		}
	}

	// Validate parameter types and collect warnings
	if a.warnings != nil {
		for i := range params {
			ValidateParameterType(&params[i], a.warnings, "")
		}
	}

	// Extract @throws error responses — resolve their types
	var resolvedErrors []ErrorResponse
	for _, er := range errorResponses {
		// Try to resolve the type name through the walker's registry
		if resolvedType, ok := a.registry.Types[er.TypeName]; ok {
			er.Type = *resolvedType
		}
		resolvedErrors = append(resolvedErrors, er)
	}

	return &Route{
		Method:         httpMethod,
		Path:           fullPath,
		OperationID:    operationID,
		Parameters:     params,
		ReturnType:     returnType,
		StatusCode:     statusCode,
		Summary:        summary,
		Description:    description,
		Deprecated:     deprecated,
		Tags:           tags,
		Security:       security,
		ErrorResponses: resolvedErrors,
		Version:        version,
	}
}

// analyzeParameter extracts parameter information from a decorated parameter.
// Returns nil if the parameter has no recognized NestJS decorator.
//
// Resolution order for determining parameter category:
//  1. Built-in NestJS decorators (@Body, @Param, @Query, @Headers) — hardcoded
//  2. Custom decorators with @in JSDoc on their declaration site — resolved via checker
//  3. No match → silently skip (correct for @CurrentUser, @Ip, etc.)
func (a *ControllerAnalyzer) analyzeParameter(paramNode *ast.Node) *RouteParameter {
	paramDecl := paramNode.AsParameterDeclaration()

	// Look for parameter decorators
	category := ""
	paramName := ""

	for _, dec := range paramNode.Decorators() {
		info := ParseDecorator(dec)
		if info == nil {
			continue
		}

		switch info.Name {
		case "Body":
			category = "body"
			if len(info.Args) > 0 {
				paramName = info.Args[0]
			}
		case "Query":
			category = "query"
			if len(info.Args) > 0 {
				paramName = info.Args[0]
			}
		case "Param":
			category = "param"
			if len(info.Args) > 0 {
				paramName = info.Args[0]
			}
		case "Headers":
			category = "headers"
			if len(info.Args) > 0 {
				paramName = info.Args[0]
			}
		case "Req", "Request", "Res", "Response":
			// Skip — raw request/response objects, not API parameters
			return nil
		default:
			// Try resolving @in JSDoc on the decorator's declaration site
			if resolved := a.resolveDecoratorIn(dec); resolved != "" {
				category = resolved
				if len(info.Args) > 0 {
					paramName = info.Args[0]
				}
			}
		}
	}

	if category == "" {
		return nil
	}

	// Extract parameter type
	var paramType metadata.Metadata
	if paramDecl.Type != nil {
		paramType = a.walker.WalkTypeNode(paramDecl.Type)
	} else {
		// Try to infer from checker
		sym := a.checker.GetSymbolAtLocation(paramNode)
		if sym != nil {
			t := shimchecker.Checker_getTypeOfSymbol(a.checker, sym)
			paramType = a.walker.WalkType(t)
		} else {
			paramType = metadata.Metadata{Kind: metadata.KindAny}
		}
	}

	// Determine if required (not optional, not nullable)
	required := paramDecl.QuestionToken == nil && !paramType.Optional && !paramType.Nullable

	return &RouteParameter{
		Category: category,
		Name:     paramName,
		Type:     paramType,
		Required: required,
	}
}

// resolveDecoratorIn resolves a custom decorator's parameter category by reading
// the @in JSDoc tag on the decorator's declaration site.
//
// This enables custom decorators to participate in OpenAPI generation:
//
//	/** @in param */
//	export const ExtractId = createParamDecorator(...)
//
// When @ExtractId('id') is used on a controller method parameter, tsgonest
// resolves the symbol for ExtractId, finds its declaration, reads the @in JSDoc,
// and treats it as a path parameter.
//
// Valid @in values: param, query, body, headers
func (a *ControllerAnalyzer) resolveDecoratorIn(dec *ast.Node) string {
	if dec.Kind != ast.KindDecorator {
		return ""
	}
	expr := dec.AsDecorator().Expression

	// Get the callee expression (the identifier/property-access before the call)
	var calleeNode *ast.Node
	switch expr.Kind {
	case ast.KindIdentifier:
		// @Foo (no call)
		calleeNode = expr
	case ast.KindCallExpression:
		// @Foo() or @Foo('arg')
		calleeNode = expr.AsCallExpression().Expression
	case ast.KindPropertyAccessExpression:
		// @ns.Foo (no call)
		calleeNode = expr
	default:
		return ""
	}

	// Resolve the symbol
	sym := a.checker.GetSymbolAtLocation(calleeNode)
	if sym == nil {
		return ""
	}

	// Get the value declaration
	decl := sym.ValueDeclaration
	if decl == nil {
		return ""
	}

	// Read JSDoc from the declaration.
	// For variable declarations (const Foo = ...), JSDoc is attached to the
	// VariableStatement (grandparent), not the VariableDeclaration itself.
	// Try the declaration node, then walk up to find JSDoc.
	return extractInTag(decl)
}

// extractInTag reads JSDoc from a node (or its ancestor VariableStatement)
// and returns the @in tag value if it matches a valid parameter category.
func extractInTag(node *ast.Node) string {
	// Try the node itself first, then parent chain (for VariableDeclaration → VariableStatement)
	for n := node; n != nil; n = n.Parent {
		jsdocs := n.JSDoc(nil)
		if len(jsdocs) > 0 {
			if cat := findInTag(jsdocs[len(jsdocs)-1]); cat != "" {
				return cat
			}
		}
		// Stop walking after VariableStatement or any statement-level node
		if n.Kind == ast.KindVariableStatement || n.Kind == ast.KindExportAssignment ||
			n.Kind == ast.KindFunctionDeclaration || n.Kind == ast.KindClassDeclaration {
			break
		}
	}
	return ""
}

// findInTag scans a JSDoc node's tags for @in and returns the category value.
func findInTag(jsdocNode *ast.Node) string {
	jsdoc := jsdocNode.AsJSDoc()
	if jsdoc.Tags == nil {
		return ""
	}
	for _, tagNode := range jsdoc.Tags.Nodes {
		tagName, comment := extractJSDocTagInfo(tagNode)
		if strings.ToLower(tagName) == "in" {
			cat := strings.TrimSpace(strings.ToLower(comment))
			switch cat {
			case "param", "query", "body", "headers":
				return cat
			}
		}
	}
	return ""
}

// extractReturnType extracts and unwraps the return type of a method.
// Promise<T> is unwrapped by the TypeWalker automatically.
//
// If the method has no explicit return type annotation, we return KindAny
// instead of triggering full type inference through the checker. Inferred
// return types (e.g., from Prisma service calls) can be deeply generic and
// take seconds to resolve — and they produce nothing useful for OpenAPI since
// there's no annotation to document.
func (a *ControllerAnalyzer) extractReturnType(methodNode *ast.Node, methodName string) metadata.Metadata {
	// Fast path: if no explicit return type annotation, skip expensive type inference.
	methodDecl := methodNode.AsMethodDeclaration()
	if methodDecl.Type == nil {
		if a.warnings != nil && methodName != "" {
			a.warnings.Add("", "missing-return-type",
				fmt.Sprintf("%s() has no return type annotation — response will be untyped in OpenAPI. "+
					"Add an explicit return type like Promise<YourDto> for proper documentation and serialization.", methodName))
		}
		return metadata.Metadata{Kind: metadata.KindAny}
	}

	// Use the type node directly — this is faster than going through
	// symbol → type → signatures → return type, and works correctly
	// for annotated return types.
	return a.walker.WalkTypeNode(methodDecl.Type)
}

// extractMethodJSDoc extracts OpenAPI-relevant JSDoc tags from a method declaration.
// Returns summary, description, deprecated, tags, security, error responses, and content type.
func extractMethodJSDoc(node *ast.Node) (summary string, description string, deprecated bool, tags []string, security []SecurityRequirement, errorResponses []ErrorResponse, contentType string) {
	if node == nil {
		return
	}

	jsdocs := node.JSDoc(nil)
	if len(jsdocs) == 0 {
		return
	}

	jsdoc := jsdocs[len(jsdocs)-1].AsJSDoc()

	// Extract description from JSDoc comment body
	if jsdoc.Comment != nil {
		description = extractNodeListText(jsdoc.Comment)
	}

	if jsdoc.Tags == nil {
		return
	}

	for _, tagNode := range jsdoc.Tags.Nodes {
		tagName, comment := extractJSDocTagInfo(tagNode)
		if tagName == "" {
			continue
		}

		switch strings.ToLower(tagName) {
		case "summary":
			summary = strings.TrimSpace(comment)
		case "description":
			// Override body text if explicit @description tag present
			description = strings.TrimSpace(comment)
		case "deprecated":
			deprecated = true
		case "tag":
			t := strings.TrimSpace(comment)
			if t != "" {
				tags = append(tags, t)
			}
		case "security":
			parts := strings.Fields(strings.TrimSpace(comment))
			if len(parts) >= 1 {
				sec := SecurityRequirement{Name: parts[0]}
				if len(parts) > 1 {
					sec.Scopes = parts[1:]
				}
				security = append(security, sec)
			}
		case "operationid":
			// Override the auto-generated operationId
			// (handled separately in the caller)
		case "throws":
			// @throws {400} BadRequestError
			er := parseThrowsTag(comment)
			if er != nil {
				errorResponses = append(errorResponses, *er)
			}
		case "contenttype":
			contentType = strings.TrimSpace(comment)
		}
	}

	return
}

// parseThrowsTag parses a @throws tag comment like "{400} BadRequestError" or "{401} UnauthorizedError".
func parseThrowsTag(comment string) *ErrorResponse {
	comment = strings.TrimSpace(comment)
	if !strings.HasPrefix(comment, "{") {
		return nil
	}
	closeBrace := strings.Index(comment, "}")
	if closeBrace < 0 {
		return nil
	}

	statusStr := comment[1:closeBrace]
	statusCode, err := strconv.Atoi(strings.TrimSpace(statusStr))
	if err != nil {
		return nil
	}

	typeName := strings.TrimSpace(comment[closeBrace+1:])
	if typeName == "" {
		return nil
	}

	return &ErrorResponse{
		StatusCode: statusCode,
		TypeName:   typeName,
	}
}

// httpMethodForDecorator maps a decorator name to its HTTP method.
func httpMethodForDecorator(name string) string {
	switch name {
	case "Get":
		return "GET"
	case "Post":
		return "POST"
	case "Put":
		return "PUT"
	case "Delete":
		return "DELETE"
	case "Patch":
		return "PATCH"
	case "Head":
		return "HEAD"
	case "Options":
		return "OPTIONS"
	default:
		return ""
	}
}

// defaultStatusCode returns the default HTTP status code for a method.
func defaultStatusCode(method string) int {
	switch method {
	case "POST":
		return 201
	default:
		return 200
	}
}

// deriveTag derives an OpenAPI tag from a controller class name.
// e.g., "UserController" → "User", "UsersController" → "Users"
func deriveTag(className string) string {
	if strings.HasSuffix(className, "Controller") {
		return className[:len(className)-len("Controller")]
	}
	return className
}

// MatchesGlob checks if a file path matches any of the include patterns
// and does not match any of the exclude patterns.
func MatchesGlob(filePath string, includePatterns []string, excludePatterns []string) bool {
	if len(includePatterns) == 0 {
		return false
	}

	// Normalize path separators
	filePath = filepath.ToSlash(filePath)

	// Check exclude first
	for _, pattern := range excludePatterns {
		pattern = filepath.ToSlash(pattern)
		if globMatch(filePath, pattern) {
			return false
		}
	}

	// Check include
	for _, pattern := range includePatterns {
		pattern = filepath.ToSlash(pattern)
		if globMatch(filePath, pattern) {
			return true
		}
	}

	return false
}

// globMatch matches a path against a glob pattern with ** support.
// The matching is done against suffixes of the path — if the pattern
// is "src/**/*.controller.ts", it matches any file under a "src/" directory
// whose name matches "*.controller.ts".
func globMatch(filePath, pattern string) bool {
	// Try exact match first
	if matched, _ := filepath.Match(pattern, filePath); matched {
		return true
	}

	// Handle ** glob patterns
	if strings.Contains(pattern, "**") {
		parts := strings.SplitN(pattern, "**", 2)
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")

		if prefix == "" {
			// Pattern like **/*.controller.ts — match suffix against any file
			if suffix == "" {
				return true
			}
			fileName := filepath.Base(filePath)
			if matched, _ := filepath.Match(suffix, fileName); matched {
				return true
			}
		} else {
			// Pattern like src/**/*.controller.ts — find prefix in path, then match suffix
			searchStr := "/" + prefix + "/"
			idx := strings.Index(filePath, searchStr)
			if idx >= 0 {
				remaining := filePath[idx+len(searchStr):]
				if suffix == "" {
					return true
				}
				fileName := filepath.Base(remaining)
				if matched, _ := filepath.Match(suffix, fileName); matched {
					return true
				}
				if matched, _ := filepath.Match(suffix, remaining); matched {
					return true
				}
			}
		}
	} else {
		// No ** — try matching just the basename
		baseName := filepath.Base(filePath)
		patternBase := filepath.Base(pattern)
		if matched, _ := filepath.Match(patternBase, baseName); matched {
			return true
		}
	}

	return false
}

// parseNumericLiteral converts a numeric literal string to float64.
// Used for extracting values from decorator arguments like @HttpCode(201).
func parseNumericLiteral(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

// Ensure strconv is used (it's used in nestjs.go in this package)
var _ = strconv.ParseFloat

// ValidateParameterType checks that a route parameter has a valid type for its category
// and records warnings via the WarningCollector if not.
func ValidateParameterType(param *RouteParameter, wc *WarningCollector, sourceFile string) {
	if wc == nil {
		return
	}
	switch param.Category {
	case "param":
		// Path params must be scalar (string, number). Warn if arrays/objects are used.
		if param.Type.Kind == metadata.KindObject || param.Type.Kind == metadata.KindArray {
			wc.Add(sourceFile, "param-non-scalar",
				fmt.Sprintf("path parameter %q has non-scalar type %q; only string/number are valid", param.Name, param.Type.Kind))
		}
	case "query":
		// Query params should be flat (no deeply nested objects).
		if hasNestedObjects(&param.Type) {
			wc.Add(sourceFile, "query-complex-type",
				fmt.Sprintf("query parameter %q has nested object type; query params should be flat", param.Name))
		}
	case "headers":
		// Headers shouldn't be null.
		if param.Type.Nullable {
			wc.Add(sourceFile, "header-null",
				fmt.Sprintf("header parameter %q is nullable; header values cannot be null", param.Name))
		}
	}
}

// hasNestedObjects checks if a metadata type contains nested object types.
// Returns true if the given type is an object with at least one property
// whose type is also an object (i.e. nested objects).
func hasNestedObjects(m *metadata.Metadata) bool {
	if m.Kind == metadata.KindObject {
		for _, prop := range m.Properties {
			if prop.Type.Kind == metadata.KindObject {
				return true
			}
		}
	}
	return false
}
