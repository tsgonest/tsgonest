package openapi

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// Document represents an OpenAPI 3.2 document.
type Document struct {
	OpenAPI    string                `json:"openapi"`
	Info       Info                  `json:"info"`
	Servers    []Server              `json:"servers,omitempty"`
	Paths      map[string]*PathItem  `json:"paths"`
	Components *Components           `json:"components,omitempty"`
	Tags       []Tag                 `json:"tags,omitempty"`
	Security   []map[string][]string `json:"security,omitempty"`
}

// Info holds API metadata.
type Info struct {
	Title          string   `json:"title"`
	Description    string   `json:"description,omitempty"`
	Version        string   `json:"version"`
	TermsOfService string   `json:"termsOfService,omitempty"`
	Contact        *Contact `json:"contact,omitempty"`
	License        *License `json:"license,omitempty"`
}

// Contact holds API contact info.
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License holds API license info.
type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Server represents an OpenAPI server.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// PathItem holds the operations for a single path.
type PathItem struct {
	Get     *Operation `json:"get,omitempty"`
	Post    *Operation `json:"post,omitempty"`
	Put     *Operation `json:"put,omitempty"`
	Delete  *Operation `json:"delete,omitempty"`
	Patch   *Operation `json:"patch,omitempty"`
	Head    *Operation `json:"head,omitempty"`
	Options *Operation `json:"options,omitempty"`
}

// Operation represents an HTTP operation.
type Operation struct {
	OperationID         string                `json:"operationId,omitempty"`
	Summary             string                `json:"summary,omitempty"`
	Description         string                `json:"description,omitempty"`
	Tags                []string              `json:"tags,omitempty"`
	Deprecated          bool                  `json:"deprecated,omitempty"`
	Security            []map[string][]string `json:"security,omitempty"`
	Parameters          []Parameter           `json:"parameters,omitempty"`
	RequestBody         *RequestBody          `json:"requestBody,omitempty"`
	Responses           Responses             `json:"responses"`
	XTsgonestController string                `json:"x-tsgonest-controller,omitempty"`
	XTsgonestMethod     string                `json:"x-tsgonest-method,omitempty"`
	// Extensions holds vendor extension properties (x-* keys) from @extension JSDoc.
	// These are serialized as top-level fields in the operation JSON.
	Extensions map[string]string `json:"-"` // excluded from default marshaling
}

// MarshalJSON implements custom JSON marshaling to:
//  1. Include vendor extensions (x-* keys) as top-level fields.
//  2. Serialize an explicitly empty Security slice as "security": [] (for @public routes).
//     Go's omitempty omits both nil and empty slices, but OpenAPI requires [] to override global security.
func (op Operation) MarshalJSON() ([]byte, error) {
	// Alias to avoid infinite recursion
	type OperationAlias Operation
	data, err := json.Marshal(OperationAlias(op))
	if err != nil {
		return nil, err
	}

	// Check if we need post-processing: empty-but-non-nil security or vendor extensions
	needsEmptySecurity := op.Security != nil && len(op.Security) == 0
	needsExtensions := len(op.Extensions) > 0

	if !needsEmptySecurity && !needsExtensions {
		return data, nil
	}

	// Unmarshal into raw map for modification
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	// Inject "security": [] for @public routes
	if needsEmptySecurity {
		raw["security"] = json.RawMessage(`[]`)
	}

	// Merge vendor extensions
	for k, v := range op.Extensions {
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		raw[k] = encoded
	}

	return json.Marshal(raw)
}

// Parameter represents an OpenAPI parameter (query, path, header).
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // "query", "path", "header"
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required"`
	Style       string  `json:"style,omitempty"`   // "form", "simple", "deepObject", etc.
	Explode     *bool   `json:"explode,omitempty"` // true for repeated query params (?a=1&a=2)
	Schema      *Schema `json:"schema"`
}

// RequestBody represents an OpenAPI request body.
type RequestBody struct {
	Required bool                 `json:"required"`
	Content  map[string]MediaType `json:"content"`
}

// MediaType holds the schema for a content type.
type MediaType struct {
	Schema     *Schema `json:"schema,omitempty"`
	ItemSchema *Schema `json:"itemSchema,omitempty"` // OpenAPI 3.2: per-item schema for streaming (SSE, JSONL)
}

// Responses maps status codes to response objects.
type Responses map[string]*Response

// Response represents an OpenAPI response.
type Response struct {
	Description string                   `json:"description"`
	Headers     map[string]*HeaderObject `json:"headers,omitempty"`
	Content     map[string]MediaType     `json:"content,omitempty"`
}

// HeaderObject represents an OpenAPI Header Object.
type HeaderObject struct {
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema"`
}

// Components holds reusable schemas.
type Components struct {
	Schemas         map[string]*Schema         `json:"schemas,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme represents an OpenAPI security scheme.
type SecurityScheme struct {
	Type         string `json:"type"`                   // "http", "apiKey", "oauth2", "openIdConnect"
	Scheme       string `json:"scheme,omitempty"`       // "bearer", "basic"
	BearerFormat string `json:"bearerFormat,omitempty"` // "JWT"
	In           string `json:"in,omitempty"`           // "header", "query", "cookie" (for apiKey)
	Name         string `json:"name,omitempty"`         // header/query/cookie name (for apiKey)
	Description  string `json:"description,omitempty"`
}

// DocumentConfig holds configuration to apply to an OpenAPI document.
type DocumentConfig struct {
	Title           string
	Description     string
	Version         string
	TermsOfService  string
	Contact         *Contact
	License         *License
	Servers         []Server
	SecuritySchemes map[string]*SecurityScheme
	// Security defines global security requirements for all operations.
	Security []map[string][]string
	// Tags defines tag descriptions (merged with auto-collected tags).
	Tags []Tag
}

// Tag represents an OpenAPI tag.
type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// GenerateOptions holds options for global prefix and versioning transforms.
type GenerateOptions struct {
	GlobalPrefix   string
	VersioningType string // "URI", "HEADER", "MEDIA_TYPE", ""
	VersionPrefix  string // default "v" for URI versioning
	DefaultVersion string
}

// Generator creates OpenAPI documents from controller analysis results.
type Generator struct {
	schemaGen *SchemaGenerator
}

// NewGenerator creates a new OpenAPI generator.
func NewGenerator(registry *metadata.TypeRegistry) *Generator {
	return &Generator{
		schemaGen: NewSchemaGenerator(registry),
	}
}

// Generate creates an OpenAPI 3.2 document from a list of controllers.
func (g *Generator) Generate(controllers []analyzer.ControllerInfo) *Document {
	return g.GenerateWithOptions(controllers, nil)
}

// buildOperation creates an Operation from a Route.
func (g *Generator) buildOperation(route analyzer.Route, controllerName string) *Operation {
	op := &Operation{
		OperationID:         route.OperationID,
		Summary:             route.Summary,
		Description:         route.Description,
		Tags:                route.Tags,
		Deprecated:          route.Deprecated,
		Responses:           make(Responses),
		XTsgonestController: controllerName,
		XTsgonestMethod:     route.MethodName,
	}

	// Map vendor extensions from @extension JSDoc
	if len(route.Extensions) > 0 {
		op.Extensions = route.Extensions
	}

	// Map security requirements.
	// @public → empty security array (overrides global security).
	// Per-route @security → explicit security requirements.
	// No annotation → inherits global security (omit from operation).
	if route.IsPublic {
		op.Security = []map[string][]string{} // empty array = no security
	} else if len(route.Security) > 0 {
		for _, sec := range route.Security {
			scopes := sec.Scopes
			if scopes == nil {
				scopes = []string{}
			}
			op.Security = append(op.Security, map[string][]string{sec.Name: scopes})
		}
	}

	// Process parameters
	for _, param := range route.Parameters {
		switch param.Category {
		case "body":
			// Body parameters become requestBody
			schema := g.schemaGen.MetadataToSchema(&param.Type)
			contentType := "application/json"
			if param.ContentType != "" {
				contentType = param.ContentType
			} else if param.Type.Kind == metadata.KindAtomic && param.Type.Atomic == "string" {
				contentType = "text/plain"
			}
			op.RequestBody = &RequestBody{
				Required: param.Required,
				Content: map[string]MediaType{
					contentType: {Schema: schema},
				},
			}

		case "query":
			// Query parameters: if the type is an object, decompose into individual params
			if param.Type.Kind == metadata.KindObject && param.Name == "" {
				g.decomposeQueryObject(op, &param.Type)
			} else if param.Type.Kind == metadata.KindRef && param.Name == "" {
				// Resolve ref and decompose
				if resolved, ok := g.schemaGen.registry.Types[param.Type.Ref]; ok {
					g.decomposeQueryObject(op, resolved)
				} else {
					op.Parameters = append(op.Parameters, Parameter{
						Name:        param.Name,
						In:          "query",
						Description: param.Description,
						Required:    param.Required,
						Schema:      g.schemaGen.MetadataToSchema(&param.Type),
					})
				}
			} else {
				name := param.Name
				if name == "" {
					name = "query"
				}
				op.Parameters = append(op.Parameters, Parameter{
					Name:        name,
					In:          "query",
					Description: param.Description,
					Required:    param.Required,
					Schema:      g.schemaGen.MetadataToSchema(&param.Type),
				})
			}

		case "param":
			name := param.Name
			if name == "" {
				name = "id" // fallback
			}
			op.Parameters = append(op.Parameters, Parameter{
				Name:        name,
				In:          "path",
				Description: param.Description,
				Required:    true, // path params are always required
				Schema:      g.schemaGen.MetadataToSchema(&param.Type),
			})

		case "headers":
			// Headers: if the type is an object and no field name, decompose into individual header params
			if param.Type.Kind == metadata.KindObject && param.Name == "" {
				g.decomposeHeaderObject(op, &param.Type)
			} else if param.Type.Kind == metadata.KindRef && param.Name == "" {
				if resolved, ok := g.schemaGen.registry.Types[param.Type.Ref]; ok {
					g.decomposeHeaderObject(op, resolved)
				} else {
					op.Parameters = append(op.Parameters, Parameter{
						Name:        param.Name,
						In:          "header",
						Description: param.Description,
						Required:    param.Required,
						Schema:      g.schemaGen.MetadataToSchema(&param.Type),
					})
				}
			} else {
				name := param.Name
				if name == "" {
					name = "headers"
				}
				op.Parameters = append(op.Parameters, Parameter{
					Name:        name,
					In:          "header",
					Description: param.Description,
					Required:    param.Required,
					Schema:      g.schemaGen.MetadataToSchema(&param.Type),
				})
			}
		}
	}

	// Build success response
	statusStr := statusCodeString(route.StatusCode)

	// Determine response description: @Returns description > auto-generated
	respDescription := statusDescription(route.StatusCode)
	if route.ResponseDescription != "" {
		respDescription = route.ResponseDescription
	}

	// Determine response content type: @Returns contentType > "application/json"
	respContentType := "application/json"
	if route.ResponseContentType != "" {
		respContentType = route.ResponseContentType
	}

	if route.IsSSE {
		// SSE endpoint → text/event-stream with itemSchema (OpenAPI 3.2)
		eventSchema := g.buildSSEEventSchema(&route.ReturnType)
		op.Responses[statusStr] = &Response{
			Description: "Server-Sent Events stream",
			Content: map[string]MediaType{
				"text/event-stream": {ItemSchema: eventSchema},
			},
		}
	} else if route.ReturnType.Kind == metadata.KindVoid {
		op.Responses[statusStr] = &Response{
			Description: respDescription,
		}
	} else {
		responseSchema := g.schemaGen.MetadataToSchema(&route.ReturnType)
		op.Responses[statusStr] = &Response{
			Description: respDescription,
			Content: map[string]MediaType{
				respContentType: {Schema: responseSchema},
			},
		}
	}

	// Add additional success responses from multiple @Returns<T>() decorators
	for _, ar := range route.AdditionalResponses {
		arStatusStr := fmt.Sprintf("%d", ar.StatusCode)
		arContentType := "application/json"
		if ar.ContentType != "" {
			arContentType = ar.ContentType
		}
		arDescription := statusDescription(ar.StatusCode)
		if ar.Description != "" {
			arDescription = ar.Description
		}
		if ar.ReturnType.Kind == metadata.KindVoid {
			op.Responses[arStatusStr] = &Response{Description: arDescription}
		} else {
			arSchema := g.schemaGen.MetadataToSchema(&ar.ReturnType)
			op.Responses[arStatusStr] = &Response{
				Description: arDescription,
				Content:     map[string]MediaType{arContentType: {Schema: arSchema}},
			}
		}
	}

	// Add error responses from @throws
	for _, er := range route.ErrorResponses {
		errStatusStr := fmt.Sprintf("%d", er.StatusCode)
		errDescription := statusDescription(er.StatusCode)
		if er.Description != "" {
			errDescription = er.Description
		}
		if er.Type.Kind != "" {
			errSchema := g.schemaGen.MetadataToSchema(&er.Type)
			op.Responses[errStatusStr] = &Response{
				Description: errDescription,
				Content: map[string]MediaType{
					"application/json": {Schema: errSchema},
				},
			}
		} else {
			op.Responses[errStatusStr] = &Response{
				Description: errDescription,
			}
		}
	}

	// Add @Header() response headers to the primary success response
	if len(route.ResponseHeaders) > 0 {
		resp := op.Responses[statusStr]
		if resp != nil {
			if resp.Headers == nil {
				resp.Headers = make(map[string]*HeaderObject)
			}
			for _, h := range route.ResponseHeaders {
				resp.Headers[h.Name] = &HeaderObject{
					Schema: &Schema{Type: "string", Enum: []interface{}{h.Value}},
				}
			}
		}
	}

	// Add @Redirect() response — adds a 3xx response with Location header
	if route.Redirect != nil {
		redirectStatus := fmt.Sprintf("%d", route.Redirect.StatusCode)
		redirectDesc := statusDescription(route.Redirect.StatusCode)
		headers := map[string]*HeaderObject{
			"Location": {
				Description: "Redirect target URL",
				Schema:      &Schema{Type: "string"},
			},
		}
		if route.Redirect.URL != "" {
			headers["Location"].Schema.Enum = []interface{}{route.Redirect.URL}
		}
		op.Responses[redirectStatus] = &Response{
			Description: redirectDesc,
			Headers:     headers,
		}
	}

	return op
}

// decomposeQueryObject breaks an object type into individual query parameters.
// Array-typed properties get style: "form" and explode: true so that
// repeated query params like ?status=A&status=B are properly documented.
func (g *Generator) decomposeQueryObject(op *Operation, m *metadata.Metadata) {
	for _, prop := range m.Properties {
		propSchema := g.schemaGen.MetadataToSchema(&prop.Type)
		param := Parameter{
			Name:     prop.Name,
			In:       "query",
			Required: prop.Required,
			Schema:   propSchema,
		}
		// Array query params: use style=form, explode=true for ?status=A&status=B
		if isArrayKind(&prop.Type) {
			param.Style = "form"
			explode := true
			param.Explode = &explode
		}
		op.Parameters = append(op.Parameters, param)
	}
}

// decomposeHeaderObject breaks an object type into individual header parameters.
func (g *Generator) decomposeHeaderObject(op *Operation, m *metadata.Metadata) {
	for _, prop := range m.Properties {
		propSchema := g.schemaGen.MetadataToSchema(&prop.Type)
		op.Parameters = append(op.Parameters, Parameter{
			Name:     prop.Name,
			In:       "header",
			Required: prop.Required,
			Schema:   propSchema,
		})
	}
}

// isArrayKind checks if a metadata type is an array (directly or via ref).
func isArrayKind(m *metadata.Metadata) bool {
	return m.Kind == metadata.KindArray
}

// buildSSEEventSchema creates the OpenAPI 3.2 Server-Sent Events event schema.
// The standard SSE event has: data (required), event, id, retry.
//
// If the return type resolves to something other than NestJS's MessageEvent
// (i.e., a custom DTO), we use contentMediaType + contentSchema on the data
// field so consumers know the JSON structure inside the data string.
func (g *Generator) buildSSEEventSchema(returnType *metadata.Metadata) *Schema {
	// Build the data property
	dataProp := &Schema{Type: "string"}

	// Check if the return type is a useful DTO (not just MessageEvent or any)
	// NestJS's MessageEvent has properties: data, type, id, retry — it's the envelope itself.
	// If the walker resolved to an object named "MessageEvent" or to KindAny,
	// use the generic SSE schema. Otherwise, add contentSchema for typed data.
	if returnType != nil && !isMessageEventType(returnType) && returnType.Kind != metadata.KindAny && returnType.Kind != metadata.KindVoid && returnType.Kind != "" {
		dataSchema := g.schemaGen.MetadataToSchema(returnType)
		dataProp.ContentMediaType = "application/json"
		dataProp.ContentSchema = dataSchema
	}

	return &Schema{
		Type:     "object",
		Required: []string{"data"},
		Properties: map[string]*Schema{
			"data":  dataProp,
			"event": {Type: "string"},
			"id":    {Type: "string"},
			"retry": {Type: "integer", Minimum: floatPtr(0)},
		},
	}
}

// isMessageEventType checks if a metadata type represents NestJS's MessageEvent interface.
// We detect this by name — if the resolved type is an object named "MessageEvent",
// it's the generic NestJS SSE envelope, not a user-defined DTO.
func isMessageEventType(m *metadata.Metadata) bool {
	if m.Name == "MessageEvent" {
		return true
	}
	// Also check Ref — the type might be registered as a $ref
	if m.Kind == metadata.KindRef && m.Ref == "MessageEvent" {
		return true
	}
	return false
}

// floatPtr returns a pointer to a float64 value.
func floatPtr(f float64) *float64 {
	return &f
}

// ToJSON serializes the document to JSON with indentation.
func (doc *Document) ToJSON() ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}

// ApplyConfig applies document-level configuration overrides.
func (doc *Document) ApplyConfig(cfg DocumentConfig) {
	if cfg.Title != "" {
		doc.Info.Title = cfg.Title
	}
	if cfg.Description != "" {
		doc.Info.Description = cfg.Description
	}
	if cfg.Version != "" {
		doc.Info.Version = cfg.Version
	}
	if cfg.TermsOfService != "" {
		doc.Info.TermsOfService = cfg.TermsOfService
	}
	if cfg.Contact != nil {
		doc.Info.Contact = cfg.Contact
	}
	if cfg.License != nil {
		doc.Info.License = cfg.License
	}
	if len(cfg.Servers) > 0 {
		doc.Servers = cfg.Servers
	}
	if len(cfg.SecuritySchemes) > 0 {
		if doc.Components == nil {
			doc.Components = &Components{}
		}
		doc.Components.SecuritySchemes = cfg.SecuritySchemes
	}
	if len(cfg.Security) > 0 {
		doc.Security = cfg.Security
	}
	// Merge config tag descriptions with auto-collected tags
	if len(cfg.Tags) > 0 {
		tagDescMap := make(map[string]string)
		for _, t := range cfg.Tags {
			tagDescMap[t.Name] = t.Description
		}
		// Update existing tags with descriptions
		for i := range doc.Tags {
			if desc, ok := tagDescMap[doc.Tags[i].Name]; ok {
				doc.Tags[i].Description = desc
				delete(tagDescMap, doc.Tags[i].Name)
			}
		}
		// Add config-only tags that weren't already collected from routes
		for name, desc := range tagDescMap {
			doc.Tags = append(doc.Tags, Tag{Name: name, Description: desc})
		}
		// Re-sort to maintain alphabetical order
		sort.Slice(doc.Tags, func(i, j int) bool { return doc.Tags[i].Name < doc.Tags[j].Name })
	}
}

// GenerateWithOptions creates an OpenAPI 3.1 document with global prefix and versioning applied.
// It iterates over controllers and routes, building paths with version and prefix transforms
// applied during path construction for correct per-route versioning support.
func (g *Generator) GenerateWithOptions(controllers []analyzer.ControllerInfo, opts *GenerateOptions) *Document {
	doc := &Document{
		OpenAPI: "3.2.0",
		Info: Info{
			Title:   "API",
			Version: "1.0.0",
		},
		Paths: make(map[string]*PathItem),
	}

	// Collect all unique tags
	tagSet := make(map[string]bool)

	for _, ctrl := range controllers {
		// Skip controllers annotated with @tsgonest-ignore openapi, @hidden, or @exclude
		if ctrl.IgnoreOpenAPI {
			continue
		}
		for _, route := range ctrl.Routes {
			// Determine the set of versions to iterate.
			// @Version(['1', '2']) produces multiple versioned paths.
			routeVersions := route.Versions
			if len(routeVersions) == 0 {
				routeVersions = []string{route.Version}
			}

			for _, version := range routeVersions {
				// Convert NestJS-style path params (:id) to OpenAPI-style ({id})
				openapiPath := convertPath(route.Path)

				// Apply URI versioning
				if opts != nil && opts.VersioningType == "URI" {
					v := version
					if v == "" {
						v = opts.DefaultVersion
					}
					if v != "" {
						vPrefix := opts.VersionPrefix
						if vPrefix == "" {
							vPrefix = "v"
						}
						openapiPath = "/" + vPrefix + v + openapiPath
					}
				}

				// Apply global prefix
				if opts != nil && opts.GlobalPrefix != "" {
					prefix := "/" + strings.Trim(opts.GlobalPrefix, "/")
					openapiPath = prefix + openapiPath
				}

				pathItem, exists := doc.Paths[openapiPath]
				if !exists {
					pathItem = &PathItem{}
					doc.Paths[openapiPath] = pathItem
				}

				// Create operation
				op := g.buildOperation(route, ctrl.Name)

				// Synthesize missing path parameters.
				// Controller-level params (e.g., @Controller(':workspaceID')) appear in
				// the URL template but may not have explicit @Param() decorators on the
				// method.  OpenAPI requires every {param} in the path to have a
				// corresponding parameter entry.
				ensurePathParams(op, openapiPath)

				// For HEADER versioning, add a version header parameter
				if opts != nil && opts.VersioningType == "HEADER" {
					v := version
					if v == "" {
						v = opts.DefaultVersion
					}
					if v != "" {
						op.Parameters = append(op.Parameters, Parameter{
							Name:     "X-API-Version",
							In:       "header",
							Required: true,
							Schema:   &Schema{Type: "string", Const: v},
						})
					}
				}

				// Set operation on the correct method
				switch route.Method {
				case "GET":
					pathItem.Get = op
				case "POST":
					pathItem.Post = op
				case "PUT":
					pathItem.Put = op
				case "DELETE":
					pathItem.Delete = op
				case "PATCH":
					pathItem.Patch = op
				case "HEAD":
					pathItem.Head = op
				case "OPTIONS":
					pathItem.Options = op
				}

				// Collect tags
				for _, tag := range route.Tags {
					tagSet[tag] = true
				}
			}
		}
	}

	// Add tags to document
	var tags []Tag
	for tag := range tagSet {
		tags = append(tags, Tag{Name: tag})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	if len(tags) > 0 {
		doc.Tags = tags
	}

	// Add component schemas
	schemas := g.schemaGen.Schemas()
	if len(schemas) > 0 {
		doc.Components = &Components{Schemas: schemas}
	}

	return doc
}

// convertPath converts NestJS-style path params to OpenAPI-style.
// e.g., "/users/:id" → "/users/{id}"
func convertPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + part[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

// pathParamRe matches OpenAPI-style path parameters like {id} or {workspaceID}.
var pathParamRe = regexp.MustCompile(`\{([^}]+)\}`)

// ensurePathParams guarantees that every {param} placeholder in the OpenAPI
// path has a corresponding entry in op.Parameters with "in":"path".
// Controller-level route params (e.g. @Controller(':workspaceID')) produce
// {workspaceID} in the path but may lack an explicit @Param() decorator on
// the method; this function synthesises the missing parameter entries so the
// OpenAPI document is spec-compliant and the SDK generator can produce correct
// code.
func ensurePathParams(op *Operation, openapiPath string) {
	matches := pathParamRe.FindAllStringSubmatch(openapiPath, -1)
	if len(matches) == 0 {
		return
	}

	// Build a set of already-declared path param names.
	declared := make(map[string]bool, len(op.Parameters))
	for _, p := range op.Parameters {
		if p.In == "path" {
			declared[p.Name] = true
		}
	}

	for _, m := range matches {
		name := m[1]
		if declared[name] {
			continue
		}
		op.Parameters = append(op.Parameters, Parameter{
			Name:     name,
			In:       "path",
			Required: true,
			Schema:   &Schema{Type: "string"},
		})
	}
}

// statusCodeString converts an integer status code to a string.
func statusCodeString(code int) string {
	if code == 0 {
		return "200"
	}
	return fmt.Sprintf("%d", code)
}

// statusDescription returns a human-readable description for a status code.
func statusDescription(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 303:
		return "See Other"
	case 307:
		return "Temporary Redirect"
	case 308:
		return "Permanent Redirect"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	default:
		return "OK"
	}
}
