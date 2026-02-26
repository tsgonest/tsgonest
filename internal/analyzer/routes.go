package analyzer

import (
	"context"
	"fmt"
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
	// IgnoreOpenAPI is true when the controller should be excluded from OpenAPI generation.
	// Set by @tsgonest-ignore openapi, @hidden, or @exclude JSDoc on the class.
	IgnoreOpenAPI bool
}

// Route represents a single HTTP route extracted from a controller method.
type Route struct {
	// Method is the HTTP method (GET, POST, PUT, DELETE, PATCH, etc.).
	Method string
	// Path is the full route path (controller prefix + method sub-path).
	Path string
	// OperationID is the OpenAPI operationId (e.g., "User_findAll", "User_create").
	// Format: ControllerName_methodName, or overridden via @operationid JSDoc.
	OperationID string
	// MethodName is the raw TypeScript method name (e.g., "findAll", "create").
	// Used by the rewriter to locate method bodies in compiled JS output.
	MethodName string
	// Parameters holds the route parameters (@Body, @Query, @Param, @Headers).
	Parameters []RouteParameter
	// ReturnTypeName is the original return type name from source code (e.g., "UserResponse").
	ReturnTypeName string
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
	// IsPublic indicates the route explicitly opts out of global/controller security.
	// Set by @public JSDoc on the method or controller.
	IsPublic bool
	// Extensions holds vendor extension properties (x-* keys) from @extension JSDoc.
	Extensions map[string]string
	// AdditionalResponses holds extra success responses from multiple @Returns<T>() decorators.
	// The first @Returns determines the primary response; subsequent ones with different
	// status codes become additional responses.
	AdditionalResponses []AdditionalResponse
	// ResponseHeaders holds response headers from @Header() decorator.
	// Each entry maps a header name to its static value.
	ResponseHeaders []ResponseHeader
	// Redirect holds redirect information from @Redirect() decorator.
	// Non-nil when the method uses @Redirect(url, statusCode).
	Redirect *RedirectInfo
	// Versions holds multiple version strings from @Version(['1', '2']).
	// When non-empty, this replaces the single Version field and causes the route
	// to be expanded into multiple versioned paths in OpenAPI.
	Versions []string
}

// ResponseHeader represents a response header from the @Header() decorator.
type ResponseHeader struct {
	// Name is the header name (e.g., "Cache-Control").
	Name string
	// Value is the static header value (e.g., "none").
	Value string
}

// RedirectInfo holds the redirect URL and status code from @Redirect().
type RedirectInfo struct {
	// URL is the redirect target URL.
	URL string
	// StatusCode is the HTTP redirect status code (default 302).
	StatusCode int
}

// AdditionalResponse represents an additional success response from @Returns<T>().
type AdditionalResponse struct {
	StatusCode  int
	ReturnType  metadata.Metadata
	ContentType string
	Description string
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
	// Description is an optional human-readable description of the error.
	// Parsed from @throws {404} TypeName - description text
	Description string
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
	// TypeName is the original type name from the source code (e.g., "RegisterRequest").
	// Set when the parameter type is a named type alias, interface, or class.
	TypeName string
	// Type is the resolved type metadata.
	Type metadata.Metadata
	// Required indicates whether the parameter is required.
	Required bool
	// ContentType is the request body content type.
	// Default is "application/json". Other values: "text/plain", "multipart/form-data", "application/x-www-form-urlencoded".
	// Only meaningful for body parameters.
	ContentType string
	// Description is from JSDoc @param tag. Used as ParameterObject.description in OpenAPI.
	Description string
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

	// Warn for runtime-generated controllers (non-top-level classes with @Controller).
	// These are not statically analyzable and are intentionally excluded from OpenAPI.
	a.warnUnsupportedRuntimeControllers(sf)

	for _, stmt := range sf.Statements.Nodes {
		if stmt.Kind != ast.KindClassDeclaration {
			continue
		}

		infos := a.analyzeClass(stmt, sf.FileName())
		controllers = append(controllers, infos...)
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
// May return multiple ControllerInfo for @Controller(['path1', 'path2']) array paths.
func (a *ControllerAnalyzer) analyzeClass(classNode *ast.Node, sourceFile string) []ControllerInfo {
	classDecl := classNode.AsClassDeclaration()
	className := ""
	if classDecl.Name() != nil {
		className = classDecl.Name().Text()
	}

	// Look for @Controller decorator
	var controllerPaths []string
	controllerVersion := ""
	isController := false
	for _, dec := range classNode.Decorators() {
		info := ParseDecorator(dec)
		if info == nil {
			continue
		}
		isCtrl := IsControllerDecorator(info)
		if !isCtrl {
			if origName := a.resolveDecoratorOriginalName(dec); origName == "Controller" {
				isCtrl = true
			}
		}
		if isCtrl {
			pathArgs, hasPathArg, unsupported := extractStaticDecoratorPathArgs(dec)
			if unsupported {
				a.warnUnsupportedDynamicControllerPath(classNode, sourceFile, className)
				return nil
			}
			isController = true
			if hasPathArg {
				controllerPaths = pathArgs
			}
			controllerVersion = extractControllerVersion(dec)
			break
		}
	}

	if !isController {
		return nil
	}

	// Default to single empty path if no paths specified
	if len(controllerPaths) == 0 {
		controllerPaths = []string{""}
	}

	// Extract class-level JSDoc metadata (@tag, @security, @hidden, etc.)
	classInfo := extractClassJSDoc(classNode)

	// Derive tags: class-level @tag overrides auto-derived tag
	var defaultTags []string
	if len(classInfo.Tags) > 0 {
		defaultTags = classInfo.Tags
	} else {
		tag := deriveTag(className)
		if tag != "" {
			defaultTags = []string{tag}
		}
	}

	// For each controller path, analyze methods and produce a ControllerInfo.
	// Usually there's just one path, but @Controller(['v1/users', 'v2/users']) produces multiple.
	var result []ControllerInfo
	for _, controllerPath := range controllerPaths {
		var routes []Route
		if classDecl.Members != nil {
			for _, member := range classDecl.Members.Nodes {
				if member.Kind != ast.KindMethodDeclaration {
					continue
				}

				methodRoutes := a.analyzeMethod(member, controllerPath, "", className, sourceFile)
				for _, route := range methodRoutes {
					// Apply class-level defaults: tags (if method didn't override)
					if len(route.Tags) == 0 {
						route.Tags = defaultTags
					}
					// Apply class-level security (if method didn't set its own)
					if len(route.Security) == 0 && len(classInfo.Security) > 0 {
						route.Security = classInfo.Security
					}
					// Apply class-level @public (if method didn't set its own security)
					if !route.IsPublic && classInfo.IsPublic {
						route.IsPublic = true
					}
					// Apply controller-level version (if method didn't set its own @Version)
					if route.Version == "" && controllerVersion != "" {
						route.Version = controllerVersion
					}
					routes = append(routes, *route)
				}
			}
		}

		result = append(result, ControllerInfo{
			Name:          className,
			Path:          controllerPath,
			Routes:        routes,
			SourceFile:    sourceFile,
			IgnoreOpenAPI: classInfo.IgnoreOpenAPI,
		})
	}

	return result
}

// analyzeMethod attempts to parse a method declaration as a NestJS route handler.
// Returns nil if the method has no HTTP method decorator.
// May return multiple routes for @All() (expanded to all HTTP methods) or
// array path arguments (expanded to one route per path).
func (a *ControllerAnalyzer) analyzeMethod(methodNode *ast.Node, controllerPath string, tag string, className string, sourceFile string) []*Route {
	methodDecl := methodNode.AsMethodDeclaration()
	operationID := ""
	if methodDecl.Name() != nil {
		operationID = methodDecl.Name().Text()
	}

	// Look for HTTP method decorators, @HttpCode, @Version, @Sse, @Header, @Redirect, and @Returns
	httpMethod := ""
	var httpMethods []string // populated for @All()
	subPath := ""
	var subPaths []string // populated for array path args like @Get(['a', 'b'])
	statusCode := 0
	version := ""
	var versions []string
	isSSE := false
	var returnsDecoratorInfos []*DecoratorInfo
	var responseHeaders []ResponseHeader
	var redirect *RedirectInfo

	for _, dec := range methodNode.Decorators() {
		info := ParseDecorator(dec)
		if info == nil {
			continue
		}

		decName := info.Name
		if origName := a.resolveDecoratorOriginalName(dec); origName != "" {
			decName = origName
		}

		switch decName {
		case "Get", "Post", "Put", "Delete", "Patch", "Head", "Options":
			pathArgs, hasPathArg, unsupported := extractStaticDecoratorPathArgs(dec)
			if unsupported {
				a.warnUnsupportedDynamicRoutePath(methodNode, sourceFile, className, operationID, decName)
				return nil
			}
			httpMethod = httpMethodForDecorator(decName)
			if hasPathArg {
				if len(pathArgs) > 1 {
					subPaths = pathArgs
					subPath = pathArgs[0]
				} else if len(pathArgs) == 1 {
					subPath = pathArgs[0]
				}
			}
		case "All":
			pathArgs, hasPathArg, unsupported := extractStaticDecoratorPathArgs(dec)
			if unsupported {
				a.warnUnsupportedDynamicRoutePath(methodNode, sourceFile, className, operationID, decName)
				return nil
			}
			// @All() expands to all standard HTTP methods
			httpMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
			httpMethod = "GET" // placeholder for validation below
			if hasPathArg {
				if len(pathArgs) > 1 {
					subPaths = pathArgs
					subPath = pathArgs[0]
				} else if len(pathArgs) == 1 {
					subPath = pathArgs[0]
				}
			}
		case "Sse":
			// @Sse('path') — Server-Sent Events endpoint, maps to GET
			pathArg, hasPathArg, unsupported := extractStaticDecoratorPathArg(dec)
			if unsupported {
				a.warnUnsupportedDynamicRoutePath(methodNode, sourceFile, className, operationID, decName)
				return nil
			}
			httpMethod = "GET"
			isSSE = true
			if hasPathArg {
				subPath = pathArg
			}
		case "HttpCode":
			if info.NumericArg != nil {
				statusCode = int(*info.NumericArg)
			}
		case "Version":
			if len(info.Args) > 0 {
				version = info.Args[0]
			}
			// Also check for array version: @Version(['1', '2'])
			arrayVersions := extractDecoratorArrayArg(dec)
			if len(arrayVersions) > 0 {
				versions = arrayVersions
				version = arrayVersions[0]
			}
		case "Header":
			// @Header('Cache-Control', 'none') — response header
			if len(info.Args) >= 2 {
				responseHeaders = append(responseHeaders, ResponseHeader{
					Name:  info.Args[0],
					Value: info.Args[1],
				})
			}
		case "Redirect":
			// @Redirect('https://example.com', 301)
			url := ""
			code := 302 // NestJS default
			if len(info.Args) > 0 {
				url = info.Args[0]
			}
			if info.NumericArg != nil {
				code = int(*info.NumericArg)
			}
			redirect = &RedirectInfo{URL: url, StatusCode: code}
		case "Returns":
			returnsDecoratorInfos = append(returnsDecoratorInfos, info)
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
	var additionalResponses []AdditionalResponse

	if usesRawResponse && len(returnsDecoratorInfos) > 0 {
		// @Res() + @Returns<T>() — use the first type argument for primary OpenAPI response
		var statusOverride int
		returnType, responseContentType, responseDescription, statusOverride = a.extractReturnsDecoratorType(returnsDecoratorInfos[0])
		if statusOverride > 0 {
			statusCode = statusOverride
		}
		// Additional @Returns decorators become additional responses
		for _, ri := range returnsDecoratorInfos[1:] {
			rt, ct, desc, st := a.extractReturnsDecoratorType(ri)
			if st == 0 {
				st = statusCode // default to same status code
			}
			additionalResponses = append(additionalResponses, AdditionalResponse{
				StatusCode: st, ReturnType: rt, ContentType: ct, Description: desc,
			})
		}
	} else if len(returnsDecoratorInfos) > 0 {
		// @Returns<T>() without @Res() — primary response from first decorator
		var statusOverride int
		returnType, responseContentType, responseDescription, statusOverride = a.extractReturnsDecoratorType(returnsDecoratorInfos[0])
		if statusOverride > 0 {
			statusCode = statusOverride
		}
		// Additional @Returns decorators
		for _, ri := range returnsDecoratorInfos[1:] {
			rt, ct, desc, st := a.extractReturnsDecoratorType(ri)
			if st == 0 {
				st = statusCode
			}
			additionalResponses = append(additionalResponses, AdditionalResponse{
				StatusCode: st, ReturnType: rt, ContentType: ct, Description: desc,
			})
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
	summary, description, deprecated, hidden, jsdocTags, security, errorResponses, contentType, operationIDOverride, isPublic, paramDescs, methodExtensions, ignoreWarnings := extractMethodJSDoc(methodNode)

	// @hidden or @exclude — skip this route from OpenAPI generation
	if hidden {
		return nil
	}
	if len(jsdocTags) > 0 {
		tags = jsdocTags // Override controller-derived tags
	}

	// Apply @param JSDoc descriptions to parameters.
	// Match by decorator name (e.g., @Param("id")) or local variable name.
	if len(paramDescs) > 0 {
		for i := range params {
			// Try matching by decorator param name first (e.g., "id" from @Param("id"))
			if desc, ok := paramDescs[params[i].Name]; ok && desc != "" {
				params[i].Description = desc
			} else if desc, ok := paramDescs[params[i].LocalName]; ok && desc != "" {
				// Fallback: match by local variable name (e.g., "body" from `@Body() body: Dto`)
				params[i].Description = desc
			}
		}
	}

	// Save the raw TypeScript method name before transforming operationID.
	// The rewriter needs the original method name to find method bodies in JS output.
	rawMethodName := operationID

	// Build operationId: JSDoc @operationid override > ClassName_vN_methodName > ClassName_methodName
	if operationIDOverride != "" {
		operationID = operationIDOverride
	} else if className != "" {
		tag := deriveTag(className)
		if version != "" {
			operationID = tag + "_v" + version + "_" + operationID
		} else {
			operationID = tag + "_" + operationID
		}
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
		if usesRawResponse && len(returnsDecoratorInfos) == 0 && !ignoreWarnings["uses-raw-response"] {
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

	baseRoute := &Route{
		Method:              httpMethod,
		Path:                fullPath,
		OperationID:         operationID,
		MethodName:          rawMethodName,
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
		Versions:            versions,
		IsSSE:               isSSE,
		UsesRawResponse:     usesRawResponse,
		ResponseContentType: responseContentType,
		ResponseDescription: responseDescription,
		IsPublic:            isPublic,
		Extensions:          methodExtensions,
		AdditionalResponses: additionalResponses,
		ResponseHeaders:     responseHeaders,
		Redirect:            redirect,
	}

	// Expand into multiple routes for @All() and/or array paths.
	return expandRoutes(baseRoute, httpMethods, subPaths, controllerPath)
}

// expandRoutes expands a base route into multiple routes when @All() or array paths are used.
// For @All(), each HTTP method gets its own route. For array paths, each path gets its own route.
// When both are present, the cross-product is produced.
func expandRoutes(base *Route, httpMethods []string, subPaths []string, controllerPath string) []*Route {
	// No expansion needed — single method, single path
	if len(httpMethods) == 0 && len(subPaths) <= 1 {
		return []*Route{base}
	}

	// Determine methods to iterate
	methods := httpMethods
	if len(methods) == 0 {
		methods = []string{base.Method}
	}

	// Determine paths to iterate
	paths := subPaths
	if len(paths) == 0 {
		paths = []string{""} // placeholder, base.Path is already computed
	}

	var routes []*Route
	for _, method := range methods {
		for _, sp := range paths {
			r := *base // shallow copy
			r.Method = method
			if len(subPaths) > 1 {
				r.Path = CombinePaths(controllerPath, sp)
			}
			// For @All(), adjust default status code per method
			if len(httpMethods) > 0 && base.StatusCode == defaultStatusCode(base.Method) {
				r.StatusCode = defaultStatusCode(method)
			}
			routes = append(routes, &r)
		}
	}
	return routes
}

// extractDecoratorArrayArg extracts string array elements from the first argument
// of a decorator call when it is an array literal.
// e.g., @Version(['1', '2']) → ["1", "2"]
// Returns nil if the first argument is not an array literal or contains non-string elements.
func extractDecoratorArrayArg(dec *ast.Node) []string {
	if dec == nil || dec.Kind != ast.KindDecorator {
		return nil
	}
	expr := dec.AsDecorator().Expression
	if expr.Kind != ast.KindCallExpression {
		return nil
	}
	call := expr.AsCallExpression()
	if call.Arguments == nil || len(call.Arguments.Nodes) == 0 {
		return nil
	}
	arg := call.Arguments.Nodes[0]
	if arg.Kind != ast.KindArrayLiteralExpression {
		return nil
	}
	return extractArrayStringLiterals(arg)
}

// warnUnsupportedRuntimeControllers scans for @Controller classes that are not
// top-level declarations (e.g., class declarations nested inside factory
// functions). These runtime-generated controllers are intentionally excluded
// from OpenAPI because their final routes are not statically knowable.
func (a *ControllerAnalyzer) warnUnsupportedRuntimeControllers(sf *ast.SourceFile) {
	if sf == nil || a.warnings == nil {
		return
	}

	var visit ast.Visitor
	visit = func(node *ast.Node) bool {
		if node == nil {
			return false
		}

		if node.Kind == ast.KindClassDeclaration && node.Parent != nil && node.Parent.Kind != ast.KindSourceFile {
			if a.hasControllerDecorator(node) {
				a.warnUnsupportedRuntimeController(node, sf.FileName())
			}
		}

		ast.ForEachChildAndJSDoc(node, sf, visit)
		return false
	}

	for _, stmt := range sf.Statements.Nodes {
		visit(stmt)
	}
}

func (a *ControllerAnalyzer) hasControllerDecorator(node *ast.Node) bool {
	for _, dec := range node.Decorators() {
		info := ParseDecorator(dec)
		if info == nil {
			continue
		}
		if IsControllerDecorator(info) || a.resolveDecoratorOriginalName(dec) == "Controller" {
			return true
		}
	}
	return false
}

func (a *ControllerAnalyzer) warnUnsupportedRuntimeController(classNode *ast.Node, sourceFile string) {
	if a.warnings == nil || classNode == nil {
		return
	}

	loc := sourceFile
	if sf := ast.GetSourceFileOfNode(classNode); sf != nil {
		line, _ := shimscanner.GetECMALineAndCharacterOfPosition(sf, classNode.Pos())
		loc = fmt.Sprintf("%s:%d", sourceFile, line+1)
	}

	a.warnings.Add(sourceFile, "unsupported-runtime-controller",
		fmt.Sprintf("%s — runtime-generated controller detected (non-top-level @Controller class). tsgonest uses static analysis; routes are excluded from OpenAPI", loc))
}

func (a *ControllerAnalyzer) warnUnsupportedDynamicControllerPath(classNode *ast.Node, sourceFile string, className string) {
	if a.warnings == nil || classNode == nil {
		return
	}

	loc := sourceFile
	if sf := ast.GetSourceFileOfNode(classNode); sf != nil {
		line, _ := shimscanner.GetECMALineAndCharacterOfPosition(sf, classNode.Pos())
		loc = fmt.Sprintf("%s:%d", sourceFile, line+1)
	}
	if className != "" {
		loc = className + " (" + loc + ")"
	}

	a.warnings.Add(sourceFile, "unsupported-dynamic-controller-path",
		fmt.Sprintf("%s — dynamic @Controller() path is not supported by static analysis; controller is excluded from OpenAPI", loc))
}

func (a *ControllerAnalyzer) warnUnsupportedDynamicRoutePath(methodNode *ast.Node, sourceFile string, className string, methodName string, decoratorName string) {
	if a.warnings == nil || methodNode == nil {
		return
	}

	loc := methodName + "()"
	if className != "" {
		loc = className + "." + loc
	}
	if sf := ast.GetSourceFileOfNode(methodNode); sf != nil {
		line, _ := shimscanner.GetECMALineAndCharacterOfPosition(sf, methodNode.Pos())
		loc = fmt.Sprintf("%s (%s:%d)", loc, sourceFile, line+1)
	} else if sourceFile != "" {
		loc = fmt.Sprintf("%s (%s)", loc, sourceFile)
	}

	a.warnings.Add(sourceFile, "unsupported-dynamic-route-path",
		fmt.Sprintf("%s — dynamic @%s() path argument is not supported by static analysis; route is excluded from OpenAPI", loc, decoratorName))
}

// extractStaticDecoratorPathArg extracts the first decorator argument when it
// is a statically analyzable path literal.
//
// Returns:
//   - path: normalized path value
//   - hasArg: whether the decorator has a first argument
//   - unsupported: true when a first argument exists but is non-literal (e.g.,
//     identifier, call expression, template expression with interpolation)
func extractStaticDecoratorPathArg(dec *ast.Node) (path string, hasArg bool, unsupported bool) {
	if dec == nil || dec.Kind != ast.KindDecorator {
		return "", false, false
	}
	expr := dec.AsDecorator().Expression
	if expr.Kind != ast.KindCallExpression {
		return "", false, false
	}
	call := expr.AsCallExpression()
	if call.Arguments == nil || len(call.Arguments.Nodes) == 0 {
		return "", false, false
	}
	arg := call.Arguments.Nodes[0]

	switch arg.Kind {
	case ast.KindStringLiteral:
		return cleanPath(arg.AsStringLiteral().Text), true, false
	case ast.KindNoSubstitutionTemplateLiteral:
		return cleanPath(arg.AsNoSubstitutionTemplateLiteral().Text), true, false
	case ast.KindObjectLiteralExpression:
		// NestJS supports @Controller({ path: 'xxx', version: '1' })
		opts := extractControllerObjectOptions(arg)
		return opts.Path, true, false
	case ast.KindArrayLiteralExpression:
		// NestJS supports @Get(['path1', 'path2']) and @Controller(['v1/users', 'v2/users']).
		// For single-path extraction, use the first element.
		paths := extractArrayStringLiterals(arg)
		if len(paths) > 0 {
			return cleanPath(paths[0]), true, false
		}
		return "", true, true
	default:
		return "", true, true
	}
}

// extractStaticDecoratorPathArgs extracts all path arguments from a decorator.
// NestJS supports both single-string and array-of-strings for route paths:
//
//	@Get("path")        → ["path"]
//	@Get(["a", "b"])    → ["a", "b"]
//	@Controller(["x"])  → ["x"]
//
// Returns nil if no paths, or unsupported=true for dynamic args.
func extractStaticDecoratorPathArgs(dec *ast.Node) (paths []string, hasArg bool, unsupported bool) {
	if dec == nil || dec.Kind != ast.KindDecorator {
		return nil, false, false
	}
	expr := dec.AsDecorator().Expression
	if expr.Kind != ast.KindCallExpression {
		return nil, false, false
	}
	call := expr.AsCallExpression()
	if call.Arguments == nil || len(call.Arguments.Nodes) == 0 {
		return nil, false, false
	}
	arg := call.Arguments.Nodes[0]

	switch arg.Kind {
	case ast.KindStringLiteral:
		return []string{cleanPath(arg.AsStringLiteral().Text)}, true, false
	case ast.KindNoSubstitutionTemplateLiteral:
		return []string{cleanPath(arg.AsNoSubstitutionTemplateLiteral().Text)}, true, false
	case ast.KindObjectLiteralExpression:
		opts := extractControllerObjectOptions(arg)
		return []string{opts.Path}, true, false
	case ast.KindArrayLiteralExpression:
		elements := extractArrayStringLiterals(arg)
		if len(elements) == 0 {
			return nil, true, true
		}
		cleaned := make([]string, len(elements))
		for i, e := range elements {
			cleaned[i] = cleanPath(e)
		}
		return cleaned, true, false
	default:
		return nil, true, true
	}
}

// extractArrayStringLiterals extracts string literal values from an array literal expression.
// Returns nil if any element is not a string literal.
func extractArrayStringLiterals(node *ast.Node) []string {
	if node.Kind != ast.KindArrayLiteralExpression {
		return nil
	}
	arr := node.AsArrayLiteralExpression()
	if arr.Elements == nil || len(arr.Elements.Nodes) == 0 {
		return nil
	}
	var result []string
	for _, elem := range arr.Elements.Nodes {
		switch elem.Kind {
		case ast.KindStringLiteral:
			result = append(result, elem.AsStringLiteral().Text)
		case ast.KindNoSubstitutionTemplateLiteral:
			result = append(result, elem.AsNoSubstitutionTemplateLiteral().Text)
		default:
			return nil // non-literal element, bail out
		}
	}
	return result
}

// extractControllerVersion extracts the version from a @Controller() decorator
// when the argument is an object literal with a "version" property.
// Returns empty string for string-argument or no-argument forms.
func extractControllerVersion(dec *ast.Node) string {
	if dec == nil || dec.Kind != ast.KindDecorator {
		return ""
	}
	expr := dec.AsDecorator().Expression
	if expr.Kind != ast.KindCallExpression {
		return ""
	}
	call := expr.AsCallExpression()
	if call.Arguments == nil || len(call.Arguments.Nodes) == 0 {
		return ""
	}
	arg := call.Arguments.Nodes[0]
	if arg.Kind != ast.KindObjectLiteralExpression {
		return ""
	}
	return extractControllerObjectOptions(arg).Version
}

// controllerOptions holds options extracted from @Controller({ path, version }) object form.
type controllerOptions struct {
	Path    string
	Version string
}

// extractControllerObjectOptions extracts path and version from an object
// literal argument to @Controller(). Supports NestJS's object form:
// @Controller({ path: 'xxx', version: '1' })
func extractControllerObjectOptions(node *ast.Node) controllerOptions {
	opts := controllerOptions{}
	if node.Kind != ast.KindObjectLiteralExpression {
		return opts
	}
	obj := node.AsObjectLiteralExpression()
	for _, prop := range obj.Properties.Nodes {
		if prop.Kind != ast.KindPropertyAssignment {
			continue
		}
		pa := prop.AsPropertyAssignment()
		name := ""
		if pa.Name().Kind == ast.KindIdentifier {
			name = pa.Name().AsIdentifier().Text
		} else if pa.Name().Kind == ast.KindStringLiteral {
			name = pa.Name().AsStringLiteral().Text
		}
		init := pa.Initializer
		switch name {
		case "path":
			if init.Kind == ast.KindStringLiteral {
				opts.Path = cleanPath(init.AsStringLiteral().Text)
			} else if init.Kind == ast.KindNoSubstitutionTemplateLiteral {
				opts.Path = cleanPath(init.AsNoSubstitutionTemplateLiteral().Text)
			}
		case "version":
			if init.Kind == ast.KindStringLiteral {
				opts.Version = init.AsStringLiteral().Text
			} else if init.Kind == ast.KindNoSubstitutionTemplateLiteral {
				opts.Version = init.AsNoSubstitutionTemplateLiteral().Text
			}
		}
	}
	return opts
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
			// Try resolving import alias first
			if origName := a.resolveDecoratorOriginalName(dec); origName != "" {
				switch origName {
				case "Body":
					category = "body"
				case "FormDataBody":
					category = "body"
					isFormData = true
				case "Query":
					category = "query"
				case "Param":
					category = "param"
				case "Headers":
					category = "headers"
				case "Req", "Request", "Res", "Response":
					return nil
				}
				if category != "" {
					if len(info.Args) > 0 {
						paramName = info.Args[0]
					}
				}
			}
			// If alias didn't resolve, try @in JSDoc on the decorator's declaration site
			if category == "" {
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
	var paramTypeName string
	if paramDecl.Type != nil {
		paramType = a.walker.WalkTypeNode(paramDecl.Type)
		// Extract the type name from the type annotation (e.g., "RegisterRequest")
		paramTypeName = resolveTypeNodeName(paramDecl.Type, a.checker)
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
		AutoEnableCoercion(&paramType)
	}

	// Determine if required (not optional, not nullable)
	required := paramDecl.QuestionToken == nil && !paramType.Optional && !paramType.Nullable

	// Extract local variable name from the parameter declaration.
	// Binding patterns (destructured params like `{ page, limit }: Dto`) don't
	// have a simple text name — skip them to avoid a panic in Node.Text().
	localName := ""
	if paramDecl.Name() != nil && paramDecl.Name().Kind == ast.KindIdentifier {
		localName = paramDecl.Name().Text()
	}

	rp := &RouteParameter{
		Category:  category,
		Name:      paramName,
		LocalName: localName,
		TypeName:  paramTypeName,
		Type:      paramType,
		Required:  required,
	}

	// @FormDataBody always uses multipart/form-data content type
	if isFormData {
		rp.ContentType = "multipart/form-data"
	}

	return rp
}

// resolveTypeNodeName extracts the type name from a type annotation AST node.
// For `body: RegisterRequest`, this returns "RegisterRequest".
// Returns empty string for anonymous/inline types.
func resolveTypeNodeName(typeNode *ast.Node, checker *shimchecker.Checker) string {
	if typeNode == nil {
		return ""
	}
	// TypeReference nodes have a TypeName child
	if typeNode.Kind == ast.KindTypeReference {
		ref := typeNode.AsTypeReferenceNode()
		if ref.TypeName != nil && ref.TypeName.Kind == ast.KindIdentifier {
			return ref.TypeName.Text()
		}
	}
	// For intersections (e.g., `Type & tags.Something`), check the first constituent
	if typeNode.Kind == ast.KindIntersectionType {
		for _, member := range typeNode.AsIntersectionTypeNode().Types.Nodes {
			if member.Kind == ast.KindTypeReference {
				ref := member.AsTypeReferenceNode()
				if ref.TypeName != nil && ref.TypeName.Kind == ast.KindIdentifier {
					return ref.TypeName.Text()
				}
			}
		}
	}
	return ""
}

// AutoEnableCoercion sets Coerce=true on number/boolean atomic properties in query/path
// parameters. HTTP query and path values arrive as strings — number and boolean types
// need automatic string→type coercion at runtime.
func AutoEnableCoercion(m *metadata.Metadata) {
	if m.Kind == metadata.KindAtomic && (m.Atomic == "number" || m.Atomic == "boolean") {
		if m.Constraints == nil {
			m.Constraints = &metadata.Constraints{}
		}
		if m.Constraints.Coerce == nil {
			b := true
			m.Constraints.Coerce = &b
		}
	}
	// For whole-object query params, enable coercion on each property.
	// Coercion is set on BOTH Property.Constraints (used by codegen) and
	// Property.Type.Constraints for consistency.
	if m.Kind == metadata.KindObject || m.Kind == metadata.KindRef {
		for i := range m.Properties {
			prop := &m.Properties[i]
			if prop.Type.Kind == metadata.KindAtomic && (prop.Type.Atomic == "number" || prop.Type.Atomic == "boolean") {
				if prop.Constraints == nil {
					prop.Constraints = &metadata.Constraints{}
				}
				if prop.Constraints.Coerce == nil {
					b := true
					prop.Constraints.Coerce = &b
				}
			}
			AutoEnableCoercion(&prop.Type)
		}
	}
	// For array query params, enable coercion on the element type
	if m.Kind == metadata.KindArray && m.ElementType != nil {
		AutoEnableCoercion(m.ElementType)
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

// resolveDecoratorOriginalName resolves the original name of an imported decorator
// when it has been aliased (e.g., `import { Body as NestBody }`).
// Returns the original name if aliased, empty string otherwise.
func (a *ControllerAnalyzer) resolveDecoratorOriginalName(dec *ast.Node) string {
	if dec.Kind != ast.KindDecorator {
		return ""
	}
	expr := dec.AsDecorator().Expression

	// Get the callee expression (the identifier before the call)
	var calleeNode *ast.Node
	switch expr.Kind {
	case ast.KindIdentifier:
		calleeNode = expr
	case ast.KindCallExpression:
		calleeNode = expr.AsCallExpression().Expression
	default:
		return ""
	}

	if calleeNode == nil || calleeNode.Kind != ast.KindIdentifier {
		return ""
	}

	sym := a.checker.GetSymbolAtLocation(calleeNode)
	if sym == nil {
		return ""
	}

	// Check if this is an alias symbol
	if sym.Flags&ast.SymbolFlagsAlias == 0 {
		return ""
	}

	// Resolve the aliased (original) symbol
	original := a.checker.GetAliasedSymbol(sym)
	if original == nil || original.Name == "" {
		return ""
	}

	return original.Name
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
		result := a.walker.WalkTypeNode(methodDecl.Type)
		// Preserve the inner type name for Promise<T> / Observable<T>
		if result.Name == "" {
			result.Name = resolveInnerTypeName(methodDecl.Type)
		}
		return result
	}

	// No annotation — infer via the checker.
	return a.inferReturnType(methodNode, className, methodName, sourceFile)
}

// resolveInnerTypeName extracts the type name from a type node, unwrapping
// Promise<T> and Observable<T> wrappers to get the inner type name.
func resolveInnerTypeName(typeNode *ast.Node) string {
	if typeNode == nil || typeNode.Kind != ast.KindTypeReference {
		return ""
	}
	ref := typeNode.AsTypeReferenceNode()
	if ref.TypeName == nil {
		return ""
	}
	// QualifiedName (e.g., Express.Multer.File) can't be resolved to a simple
	// companion name — skip it.
	if ref.TypeName.Kind != ast.KindIdentifier {
		return ""
	}
	name := ref.TypeName.Text()

	// Unwrap Promise<T> and Observable<T> to get inner type name
	if name == "Promise" || name == "Observable" {
		if ref.TypeArguments != nil && len(ref.TypeArguments.Nodes) > 0 {
			return resolveInnerTypeName(ref.TypeArguments.Nodes[0])
		}
		return ""
	}

	// Skip built-in types that shouldn't get companions
	if name == "Array" || name == "string" || name == "number" || name == "boolean" || name == "void" {
		return ""
	}

	return name
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
