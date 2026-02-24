package analyzer

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	shimscanner "github.com/microsoft/typescript-go/shim/scanner"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// returnTypeInferenceWarnThreshold is the wall-clock duration above which we
// warn the user that a method's return type inference is slow and they should
// add an explicit return type annotation for better build performance.
const returnTypeInferenceWarnThreshold = 200 * time.Millisecond

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
	// IsSSE indicates this is a Server-Sent Events endpoint (from @Sse decorator).
	// SSE endpoints use GET method and return Observable<MessageEvent>.
	IsSSE bool
	// UsesRawResponse indicates a parameter uses @Res()/@Response(), meaning
	// the developer handles the response manually. Return type is meaningless.
	UsesRawResponse bool
	// ResponseContentType overrides the content type for the success response in OpenAPI.
	// Defaults to "application/json". Set by @Returns<T>({ contentType: 'application/pdf' }).
	ResponseContentType string
	// ResponseDescription overrides the response description in OpenAPI.
	// Set by @Returns<T>({ description: 'PDF invoice' }).
	ResponseDescription string
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
	// LocalName is the local variable name from the method signature (e.g., "body", "id").
	// Used by the rewriter to inject validation at the correct parameter.
	LocalName string
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

			route := a.analyzeMethod(member, controllerPath, tag, className, sourceFile)
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
func (a *ControllerAnalyzer) analyzeMethod(methodNode *ast.Node, controllerPath string, tag string, className string, sourceFile string) *Route {
	methodDecl := methodNode.AsMethodDeclaration()

	// Look for HTTP method decorators, @HttpCode, @Version, @Sse, and @Returns
	httpMethod := ""
	subPath := ""
	statusCode := 0
	version := ""
	isSSE := false
	var returnsDecoratorInfo *DecoratorInfo

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
		case "Sse":
			// @Sse('path') — Server-Sent Events endpoint, maps to GET
			httpMethod = "GET"
			isSSE = true
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
		case "Returns":
			returnsDecoratorInfo = info
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

	// Extract parameters, detecting @Res()/@Response() usage
	var params []RouteParameter
	usesRawResponse := false
	if methodDecl.Parameters != nil {
		for _, paramNode := range methodDecl.Parameters.Nodes {
			// Check for @Res/@Response before full analysis (analyzeParameter returns nil for these)
			if hasResponseDecorator(paramNode) {
				usesRawResponse = true
			}
			param := a.analyzeParameter(paramNode, className, operationID, sourceFile, methodNode)
			if param != nil {
				params = append(params, *param)
			}
		}
	}

	// Extract return type.
	// When @Res() is used AND @Returns<T>() is present, use T for OpenAPI schema.
	// When @Res() is used without @Returns, force void.
	// When @Res() is not used, infer normally from method return type.
	var returnType metadata.Metadata
	var responseContentType string
	var responseDescription string
	if usesRawResponse && returnsDecoratorInfo != nil {
		// @Res() + @Returns<T>() — use the type argument for OpenAPI
		var statusOverride int
		returnType, responseContentType, responseDescription, statusOverride = a.extractReturnsDecoratorType(returnsDecoratorInfo)
		if statusOverride > 0 {
			statusCode = statusOverride
		}
	} else if usesRawResponse {
		returnType = metadata.Metadata{Kind: metadata.KindVoid}
	} else {
		returnType = a.extractReturnType(methodNode, className, operationID, sourceFile)
	}

	var tags []string
	if tag != "" {
		tags = []string{tag}
	}

	// Extract JSDoc metadata from the method
	summary, description, deprecated, hidden, jsdocTags, security, errorResponses, contentType, ignoreWarnings := extractMethodJSDoc(methodNode)

	// @hidden or @exclude — skip this route from OpenAPI generation
	if hidden {
		return nil
	}
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
		// Build location string for warnings: ControllerName.methodName() (file.ts:line)
		warnLocation := operationID + "()"
		if className != "" {
			warnLocation = className + "." + warnLocation
		}
		if sf := ast.GetSourceFileOfNode(methodNode); sf != nil {
			line, _ := shimscanner.GetECMALineAndCharacterOfPosition(sf, methodNode.Pos())
			warnLocation = fmt.Sprintf("%s (%s:%d)", warnLocation, sourceFile, line+1)
		} else if sourceFile != "" {
			warnLocation = fmt.Sprintf("%s (%s)", warnLocation, sourceFile)
		}

		for i := range params {
			ValidateParameterType(&params[i], a.warnings, sourceFile, warnLocation)
		}

		// Warn when @Res()/@Response() is used WITHOUT @Returns — return type cannot be determined statically.
		// When @Returns<T>() is present, we have the type info and no warning is needed.
		// When @tsgonest-ignore uses-raw-response is in JSDoc, suppress the warning.
		if usesRawResponse && returnsDecoratorInfo == nil && !ignoreWarnings["uses-raw-response"] {
			a.warnings.Add(sourceFile, "uses-raw-response",
				fmt.Sprintf("%s — uses @Res()/@Response(); response type cannot be determined statically. "+
					"The OpenAPI response will be empty (void). "+
					"To fix: add @Returns<YourType>() decorator, or suppress with /** @tsgonest-ignore uses-raw-response */", warnLocation))
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
		Method:              httpMethod,
		Path:                fullPath,
		OperationID:         operationID,
		Parameters:          params,
		ReturnType:          returnType,
		StatusCode:          statusCode,
		Summary:             summary,
		Description:         description,
		Deprecated:          deprecated,
		Tags:                tags,
		Security:            security,
		ErrorResponses:      resolvedErrors,
		Version:             version,
		IsSSE:               isSSE,
		UsesRawResponse:     usesRawResponse,
		ResponseContentType: responseContentType,
		ResponseDescription: responseDescription,
	}
}

// analyzeParameter extracts parameter information from a decorated parameter.
// Returns nil if the parameter has no recognized NestJS decorator.
//
// Resolution order for determining parameter category:
//  1. Built-in NestJS decorators (@Body, @Param, @Query, @Headers) — hardcoded
//  2. Custom decorators with @in JSDoc on their declaration site — resolved via checker
//  3. No match → silently skip (correct for @CurrentUser, @Ip, etc.)
func (a *ControllerAnalyzer) analyzeParameter(paramNode *ast.Node, className string, methodName string, sourceFile string, methodNode *ast.Node) *RouteParameter {
	paramDecl := paramNode.AsParameterDeclaration()

	// Look for parameter decorators
	category := ""
	paramName := ""
	isFormData := false
	var unresolvedDecorators []string // track custom decorators without @in

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
		case "FormDataBody":
			category = "body"
			isFormData = true
			// FormDataBody uses a multer factory arg, not a field name — no paramName to extract
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
			} else {
				unresolvedDecorators = append(unresolvedDecorators, info.Name)
			}
		}
	}

	if category == "" {
		// Custom decorators without @in are context-injection decorators
		// (e.g., @UserId, @CurrentUser, @Tenant) — silently skip them.
		// Users opt-in to OpenAPI inclusion by adding /** @in param|query|body|headers */
		// to the decorator declaration.
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

	// Auto-enable coercion for query and path parameters that are typed as
	// number or boolean. These arrive as strings from HTTP and need coercion.
	if category == "param" || category == "query" {
		autoEnableCoercion(&paramType)
	}

	// Determine if required (not optional, not nullable)
	required := paramDecl.QuestionToken == nil && !paramType.Optional && !paramType.Nullable

	// Extract local variable name from the parameter declaration
	localName := ""
	if paramDecl.Name() != nil {
		localName = paramDecl.Name().Text()
	}

	rp := &RouteParameter{
		Category:  category,
		Name:      paramName,
		LocalName: localName,
		Type:      paramType,
		Required:  required,
	}

	// @FormDataBody always uses multipart/form-data content type
	if isFormData {
		rp.ContentType = "multipart/form-data"
	}

	return rp
}

// autoEnableCoercion sets Coerce=true on number/boolean atomic properties in query/path
// parameters. HTTP query and path values arrive as strings — number and boolean types
// need automatic string→type coercion at runtime.
func autoEnableCoercion(m *metadata.Metadata) {
	if m.Kind == metadata.KindAtomic && (m.Atomic == "number" || m.Atomic == "boolean") {
		if m.Constraints == nil {
			m.Constraints = &metadata.Constraints{}
		}
		if m.Constraints.Coerce == nil {
			b := true
			m.Constraints.Coerce = &b
		}
	}
	// For whole-object query params, enable coercion on each property
	if m.Kind == metadata.KindObject || m.Kind == metadata.KindRef {
		for i := range m.Properties {
			autoEnableCoercion(&m.Properties[i].Type)
		}
	}
	// For array query params, enable coercion on the element type
	if m.Kind == metadata.KindArray && m.ElementType != nil {
		autoEnableCoercion(m.ElementType)
	}
}

// hasResponseDecorator checks if a parameter node has @Res() or @Response() decorator.
func hasResponseDecorator(paramNode *ast.Node) bool {
	for _, dec := range paramNode.Decorators() {
		info := ParseDecorator(dec)
		if info != nil && (info.Name == "Res" || info.Name == "Response") {
			return true
		}
	}
	return false
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
// When the method has an explicit return type annotation, we use WalkTypeNode
// for the fastest path. When there is no annotation, we fall back to
// checker-based inference: symbol → type → call signatures → return type.
// If checker inference exceeds returnTypeInferenceWarnThreshold, we still use
// the result but emit a warning so the user knows that method is slow to
// analyze and should add an explicit return type annotation.
func (a *ControllerAnalyzer) extractReturnType(methodNode *ast.Node, className string, methodName string, sourceFile string) metadata.Metadata {
	methodDecl := methodNode.AsMethodDeclaration()

	// Fast path: explicit return type annotation — use the type node directly.
	if methodDecl.Type != nil {
		return a.walker.WalkTypeNode(methodDecl.Type)
	}

	// No annotation — infer via the checker.
	return a.inferReturnType(methodNode, className, methodName, sourceFile)
}

// inferReturnType uses the checker to resolve the return type of a method that
// has no explicit return type annotation. It measures wall-clock time and emits
// a performance warning if inference is slow (indicating a computed/deeply
// generic return type that the user should annotate explicitly).
func (a *ControllerAnalyzer) inferReturnType(methodNode *ast.Node, className string, methodName string, sourceFile string) metadata.Metadata {
	start := time.Now()

	// GetSymbolAtLocation works on identifier nodes, not declaration nodes.
	nameNode := methodNode.AsMethodDeclaration().Name()
	if nameNode == nil {
		return metadata.Metadata{Kind: metadata.KindAny}
	}

	sym := a.checker.GetSymbolAtLocation(nameNode)
	if sym == nil {
		return metadata.Metadata{Kind: metadata.KindAny}
	}

	methodType := shimchecker.Checker_getTypeOfSymbol(a.checker, sym)
	if methodType == nil {
		return metadata.Metadata{Kind: metadata.KindAny}
	}

	sigs := shimchecker.Checker_getSignaturesOfType(a.checker, methodType, shimchecker.SignatureKindCall)
	if len(sigs) == 0 {
		return metadata.Metadata{Kind: metadata.KindAny}
	}

	// Use the last signature (overload resolution picks the implementation signature last).
	returnType := shimchecker.Checker_getReturnTypeOfSignature(a.checker, sigs[len(sigs)-1])
	if returnType == nil {
		return metadata.Metadata{Kind: metadata.KindAny}
	}

	result := a.walker.WalkType(returnType)
	elapsed := time.Since(start)

	if elapsed >= returnTypeInferenceWarnThreshold && a.warnings != nil && methodName != "" {
		// Build a descriptive location: ControllerName.methodName() (file.ts:line)
		location := methodName + "()"
		if className != "" {
			location = className + "." + location
		}

		// Get line number from the method node position
		if sf := ast.GetSourceFileOfNode(methodNode); sf != nil {
			line, _ := shimscanner.GetECMALineAndCharacterOfPosition(sf, methodNode.Pos())
			// line is 0-indexed, display as 1-indexed
			location = fmt.Sprintf("%s (%s:%d)", location, sourceFile, line+1)
		} else if sourceFile != "" {
			location = fmt.Sprintf("%s (%s)", location, sourceFile)
		}

		a.warnings.Add(sourceFile, "slow-return-type-inference",
			fmt.Sprintf("%s return type inference took %dms — consider adding an explicit return type annotation for better build performance.",
				location, elapsed.Milliseconds()))
	}

	return result
}

// extractReturnsDecoratorType extracts the response type from a @Returns<T>() decorator.
// It reads the type argument T via the checker and walks it into metadata.
// Also extracts contentType, description, and status override from the options object.
// statusOverride is 0 when no override was specified.
func (a *ControllerAnalyzer) extractReturnsDecoratorType(info *DecoratorInfo) (returnType metadata.Metadata, contentType string, description string, statusOverride int) {
	// Default: void (fallback if type arg is missing)
	returnType = metadata.Metadata{Kind: metadata.KindVoid}

	// Extract type argument T from @Returns<T>()
	if len(info.TypeArgNodes) > 0 {
		typeArgNode := info.TypeArgNodes[0]

		// Resolve the type through the checker → type walker
		t := shimchecker.Checker_getTypeFromTypeNode(a.checker, typeArgNode)
		if t != nil {
			returnType = a.walker.WalkType(t)
		} else {
			// Fallback: walk the type node directly
			returnType = a.walker.WalkTypeNode(typeArgNode)
		}
	}

	// Extract options from the object literal argument
	if info.ObjectLiteralArg != nil {
		if ct, ok := info.ObjectLiteralArg["contentType"]; ok && ct.Kind == ast.KindStringLiteral {
			contentType = ct.AsStringLiteral().Text
		}
		if desc, ok := info.ObjectLiteralArg["description"]; ok && desc.Kind == ast.KindStringLiteral {
			description = desc.AsStringLiteral().Text
		}
		if st, ok := info.ObjectLiteralArg["status"]; ok && st.Kind == ast.KindNumericLiteral {
			if num, err := strconv.ParseFloat(st.Text(), 64); err == nil {
				statusOverride = int(num)
			}
		}
	}

	return
}

// extractMethodJSDoc extracts OpenAPI-relevant JSDoc tags from a method declaration.
// Returns summary, description, deprecated, hidden, tags, security, error responses, content type,
// and a set of ignored warning kinds (from @tsgonest-ignore tags).
func extractMethodJSDoc(node *ast.Node) (summary string, description string, deprecated bool, hidden bool, tags []string, security []SecurityRequirement, errorResponses []ErrorResponse, contentType string, ignoreWarnings map[string]bool) {
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
		case "hidden", "exclude":
			hidden = true
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
		case "tsgonest-ignore":
			// @tsgonest-ignore uses-raw-response
			// Suppresses the specified warning kind.
			kind := strings.TrimSpace(comment)
			if kind != "" {
				if ignoreWarnings == nil {
					ignoreWarnings = make(map[string]bool)
				}
				ignoreWarnings[kind] = true
			}
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
// location is a pre-formatted string like "ControllerName.methodName() (file.ts:line)".
func ValidateParameterType(param *RouteParameter, wc *WarningCollector, sourceFile string, location string) {
	if wc == nil {
		return
	}
	switch param.Category {
	case "param":
		// Path params must be scalar (string, number). Warn if arrays/objects are used.
		if param.Type.Kind == metadata.KindObject || param.Type.Kind == metadata.KindArray || param.Type.Kind == metadata.KindRef {
			wc.Add(sourceFile, "param-non-scalar",
				fmt.Sprintf("%s — path parameter %q has non-scalar type %q; only string/number are valid in URL path segments",
					location, param.Name, param.Type.Kind))
		}
		// Path params cannot be any — no type info for URL segment.
		if param.Type.Kind == metadata.KindAny {
			wc.Add(sourceFile, "param-any",
				fmt.Sprintf("%s — path parameter %q has type 'any'; add a type annotation (string or number)",
					location, param.Name))
		}
		// Path params cannot be optional or nullable — URL segments can't be missing.
		if param.Type.Optional || param.Type.Nullable {
			wc.Add(sourceFile, "param-optional",
				fmt.Sprintf("%s — path parameter %q is optional/nullable; URL path segments cannot be missing",
					location, param.Name))
		}
		// Path params should not be union types across different kinds.
		if param.Type.Kind == metadata.KindUnion && len(param.Type.UnionMembers) > 1 {
			kinds := map[metadata.Kind]bool{}
			for _, m := range param.Type.UnionMembers {
				kinds[m.Kind] = true
			}
			if len(kinds) > 1 {
				wc.Add(sourceFile, "param-union",
					fmt.Sprintf("%s — path parameter %q is a union of different types; path params should be a single scalar type",
						location, param.Name))
			}
		}
		// @Param() without a field name — nestia errors on this, we warn.
		if param.Name == "" {
			wc.Add(sourceFile, "param-no-name",
				fmt.Sprintf("%s — @Param() used without a field name; use @Param('id') to name the path parameter",
					location))
		}
	case "query":
		// Query params should be flat (no deeply nested objects).
		if hasNestedObjects(&param.Type) {
			wc.Add(sourceFile, "query-complex-type",
				fmt.Sprintf("%s — query parameter %q has nested object type; query params should be flat (no nested objects)",
					location, param.Name))
		}
		// Whole-object query params should not be nullable.
		if param.Name == "" && param.Type.Nullable {
			wc.Add(sourceFile, "query-nullable",
				fmt.Sprintf("%s — query object is nullable; query parameters cannot be null",
					location))
		}
	case "headers":
		// Headers shouldn't be null.
		if param.Type.Nullable {
			wc.Add(sourceFile, "header-null",
				fmt.Sprintf("%s — header parameter %q is nullable; HTTP header values cannot be null",
					location, param.Name))
		}
		// Whole-object headers should not have nested objects.
		if param.Name == "" && hasNestedObjects(&param.Type) {
			wc.Add(sourceFile, "header-complex-type",
				fmt.Sprintf("%s — header object has nested object type; headers should be flat (no nested objects)",
					location))
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
