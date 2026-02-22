package openapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// Document represents an OpenAPI 3.1 document.
type Document struct {
	OpenAPI    string               `json:"openapi"`
	Info       Info                 `json:"info"`
	Servers    []Server             `json:"servers,omitempty"`
	Paths      map[string]*PathItem `json:"paths"`
	Components *Components          `json:"components,omitempty"`
	Tags       []Tag                `json:"tags,omitempty"`
}

// Info holds API metadata.
type Info struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version"`
	Contact     *Contact `json:"contact,omitempty"`
	License     *License `json:"license,omitempty"`
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
	OperationID string                `json:"operationId,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	Deprecated  bool                  `json:"deprecated,omitempty"`
	Security    []map[string][]string `json:"security,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   Responses             `json:"responses"`
}

// Parameter represents an OpenAPI parameter (query, path, header).
type Parameter struct {
	Name     string  `json:"name"`
	In       string  `json:"in"` // "query", "path", "header"
	Required bool    `json:"required"`
	Schema   *Schema `json:"schema"`
}

// RequestBody represents an OpenAPI request body.
type RequestBody struct {
	Required bool                 `json:"required"`
	Content  map[string]MediaType `json:"content"`
}

// MediaType holds the schema for a content type.
type MediaType struct {
	Schema *Schema `json:"schema"`
}

// Responses maps status codes to response objects.
type Responses map[string]*Response

// Response represents an OpenAPI response.
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
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
	Contact         *Contact
	License         *License
	Servers         []Server
	SecuritySchemes map[string]*SecurityScheme
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

// Generate creates an OpenAPI 3.1 document from a list of controllers.
func (g *Generator) Generate(controllers []analyzer.ControllerInfo) *Document {
	doc := &Document{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:   "API",
			Version: "1.0.0",
		},
		Paths: make(map[string]*PathItem),
	}

	// Collect all unique tags
	tagSet := make(map[string]bool)

	for _, ctrl := range controllers {
		for _, route := range ctrl.Routes {
			// Get or create path item
			// Convert NestJS-style path params (:id) to OpenAPI-style ({id})
			openapiPath := convertPath(route.Path)

			pathItem, exists := doc.Paths[openapiPath]
			if !exists {
				pathItem = &PathItem{}
				doc.Paths[openapiPath] = pathItem
			}

			// Create operation
			op := g.buildOperation(route)

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

// buildOperation creates an Operation from a Route.
func (g *Generator) buildOperation(route analyzer.Route) *Operation {
	op := &Operation{
		OperationID: route.OperationID,
		Summary:     route.Summary,
		Description: route.Description,
		Tags:        route.Tags,
		Deprecated:  route.Deprecated,
		Responses:   make(Responses),
	}

	// Map security requirements
	if len(route.Security) > 0 {
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
						Name:     param.Name,
						In:       "query",
						Required: param.Required,
						Schema:   g.schemaGen.MetadataToSchema(&param.Type),
					})
				}
			} else {
				name := param.Name
				if name == "" {
					name = "query"
				}
				op.Parameters = append(op.Parameters, Parameter{
					Name:     name,
					In:       "query",
					Required: param.Required,
					Schema:   g.schemaGen.MetadataToSchema(&param.Type),
				})
			}

		case "param":
			name := param.Name
			if name == "" {
				name = "id" // fallback
			}
			op.Parameters = append(op.Parameters, Parameter{
				Name:     name,
				In:       "path",
				Required: true, // path params are always required
				Schema:   g.schemaGen.MetadataToSchema(&param.Type),
			})

		case "headers":
			name := param.Name
			if name == "" {
				name = "headers"
			}
			op.Parameters = append(op.Parameters, Parameter{
				Name:     name,
				In:       "header",
				Required: param.Required,
				Schema:   g.schemaGen.MetadataToSchema(&param.Type),
			})
		}
	}

	// Build success response
	statusStr := statusCodeString(route.StatusCode)
	if route.ReturnType.Kind == metadata.KindVoid {
		op.Responses[statusStr] = &Response{
			Description: statusDescription(route.StatusCode),
		}
	} else {
		responseSchema := g.schemaGen.MetadataToSchema(&route.ReturnType)
		op.Responses[statusStr] = &Response{
			Description: statusDescription(route.StatusCode),
			Content: map[string]MediaType{
				"application/json": {Schema: responseSchema},
			},
		}
	}

	// Add error responses from @throws
	for _, er := range route.ErrorResponses {
		errStatusStr := fmt.Sprintf("%d", er.StatusCode)
		if er.Type.Kind != "" {
			errSchema := g.schemaGen.MetadataToSchema(&er.Type)
			op.Responses[errStatusStr] = &Response{
				Description: statusDescription(er.StatusCode),
				Content: map[string]MediaType{
					"application/json": {Schema: errSchema},
				},
			}
		} else {
			op.Responses[errStatusStr] = &Response{
				Description: statusDescription(er.StatusCode),
			}
		}
	}

	return op
}

// decomposeQueryObject breaks an object type into individual query parameters.
func (g *Generator) decomposeQueryObject(op *Operation, m *metadata.Metadata) {
	for _, prop := range m.Properties {
		propSchema := g.schemaGen.MetadataToSchema(&prop.Type)
		op.Parameters = append(op.Parameters, Parameter{
			Name:     prop.Name,
			In:       "query",
			Required: prop.Required,
			Schema:   propSchema,
		})
	}
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
}

// GenerateWithOptions creates an OpenAPI 3.1 document with global prefix and versioning applied.
// It iterates over controllers and routes, building paths with version and prefix transforms
// applied during path construction for correct per-route versioning support.
func (g *Generator) GenerateWithOptions(controllers []analyzer.ControllerInfo, opts *GenerateOptions) *Document {
	if opts == nil {
		return g.Generate(controllers)
	}

	doc := &Document{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:   "API",
			Version: "1.0.0",
		},
		Paths: make(map[string]*PathItem),
	}

	// Collect all unique tags
	tagSet := make(map[string]bool)

	for _, ctrl := range controllers {
		for _, route := range ctrl.Routes {
			// Convert NestJS-style path params (:id) to OpenAPI-style ({id})
			openapiPath := convertPath(route.Path)

			// Apply URI versioning
			if opts.VersioningType == "URI" {
				version := route.Version
				if version == "" {
					version = opts.DefaultVersion
				}
				if version != "" {
					vPrefix := opts.VersionPrefix
					if vPrefix == "" {
						vPrefix = "v"
					}
					openapiPath = "/" + vPrefix + version + openapiPath
				}
			}

			// Apply global prefix
			if opts.GlobalPrefix != "" {
				prefix := "/" + strings.Trim(opts.GlobalPrefix, "/")
				openapiPath = prefix + openapiPath
			}

			pathItem, exists := doc.Paths[openapiPath]
			if !exists {
				pathItem = &PathItem{}
				doc.Paths[openapiPath] = pathItem
			}

			// Create operation
			op := g.buildOperation(route)

			// For HEADER versioning, add a version header parameter
			if opts.VersioningType == "HEADER" {
				version := route.Version
				if version == "" {
					version = opts.DefaultVersion
				}
				if version != "" {
					op.Parameters = append(op.Parameters, Parameter{
						Name:     "X-API-Version",
						In:       "header",
						Required: true,
						Schema:   &Schema{Type: "string", Const: version},
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
