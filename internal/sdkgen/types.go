// Package sdkgen generates TypeScript SDK clients from OpenAPI specifications.
package sdkgen

// SDKDocument is the intermediate representation of an OpenAPI spec
// ready for TypeScript SDK generation.
type SDKDocument struct {
	Versions []VersionGroup
	Schemas  map[string]*SchemaNode
}

// VersionGroup represents all controllers under a specific API version.
// Version is empty string for unversioned routes.
type VersionGroup struct {
	Version     string            // e.g., "v1", "v2", "" for unversioned
	Controllers []ControllerGroup // sorted by name
}

// ControllerGroup represents a group of methods belonging to one controller.
type ControllerGroup struct {
	Name    string      // e.g., "OrdersController"
	Methods []SDKMethod // sorted by operation
}

// SDKMethod represents a single API method in the SDK.
type SDKMethod struct {
	Name           string     // e.g., "listOrders"
	HTTPMethod     string     // "GET", "POST", etc.
	Path           string     // "/orders/{id}" (full path including version prefix)
	PathParams     []SDKParam // from path parameters
	QueryParams    []SDKParam // from query parameters
	HeaderParams   []SDKParam // from header parameters
	Body           *SDKBody   // request body, nil if none
	ResponseType        string // TypeScript type string for 2xx response
	ResponseStatus      int    // e.g., 200, 201, 204
	ResponseContentType string // e.g., "application/json", "application/pdf", "text/event-stream"
	SSEEventType        string // TypeScript type for SSE event data (e.g., "OrderUpdate"), empty if untyped SSE
	IsVoid              bool   // true if 204 or no response body
	Summary        string
	Description    string
	Deprecated     bool
}

// SDKParam represents a parameter in an SDK method.
type SDKParam struct {
	Name     string // parameter name
	TSType   string // TypeScript type string
	Required bool
}

// SDKBody represents a request body.
type SDKBody struct {
	TSType      string // TypeScript type string
	Required    bool
	ContentType string // "application/json", "text/plain", etc.
}

// SchemaNode is a simplified representation of an OpenAPI schema
// used for TypeScript type generation.
type SchemaNode struct {
	// Basic type info
	Type        string // "string", "number", "integer", "boolean", "object", "array"
	Format      string // "int32", "int64", "float", "double", "date-time", "uuid", etc.
	Description string // OpenAPI description, used for JSDoc generation

	// Object properties
	Properties map[string]*SchemaNode
	Required   []string

	// Array items
	Items *SchemaNode

	// Composition
	AnyOf []*SchemaNode
	OneOf []*SchemaNode
	AllOf []*SchemaNode

	// Enum values (string literal union)
	Enum []any

	// Const value
	Const any

	// Reference
	Ref string // resolved name (without #/components/schemas/ prefix)

	// Additional properties for Record<string, T>
	AdditionalProperties *SchemaNode

	// Nullable
	Nullable bool

	// Discriminator for oneOf
	Discriminator *Discriminator

	// Name for named types (used when generating interfaces)
	Name string
}

// Discriminator holds discriminator info for oneOf schemas.
type Discriminator struct {
	PropertyName string
	Mapping      map[string]string
}
