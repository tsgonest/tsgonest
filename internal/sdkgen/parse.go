package sdkgen

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// openAPIDoc mirrors the subset of OpenAPI 3.x we need for SDK generation.
type openAPIDoc struct {
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components *openAPIComponents                    `json:"components"`
}

type openAPIComponents struct {
	Schemas map[string]json.RawMessage `json:"schemas"`
}

type openAPIOperation struct {
	OperationID         string                      `json:"operationId"`
	Summary             string                      `json:"summary"`
	Description         string                      `json:"description"`
	Deprecated          bool                        `json:"deprecated"`
	Tags                []string                    `json:"tags"`
	Parameters          []openAPIParameter          `json:"parameters"`
	RequestBody         *openAPIRequestBody         `json:"requestBody"`
	Responses           map[string]*openAPIResponse `json:"responses"`
	XTsgonestController string                      `json:"x-tsgonest-controller"`
	XTsgonestMethod     string                      `json:"x-tsgonest-method"`
}

type openAPIParameter struct {
	Name     string          `json:"name"`
	In       string          `json:"in"`
	Required bool            `json:"required"`
	Schema   json.RawMessage `json:"schema"`
}

type openAPIRequestBody struct {
	Required bool                        `json:"required"`
	Content  map[string]openAPIMediaType `json:"content"`
}

type openAPIMediaType struct {
	Schema     json.RawMessage `json:"schema"`
	ItemSchema json.RawMessage `json:"itemSchema"`
}

type openAPIResponse struct {
	Description string                      `json:"description"`
	Content     map[string]openAPIMediaType `json:"content"`
}

// versionPrefixRe matches /v{N}/ at the start of a path (e.g., /v1/, /v2/).
var versionPrefixRe = regexp.MustCompile(`^/v(\d+)(/|$)`)

// ParseOpenAPI reads an OpenAPI JSON file and transforms it into the SDK IR.
func ParseOpenAPI(path string) (*SDKDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading OpenAPI file: %w", err)
	}

	return ParseOpenAPIBytes(data)
}

// ParseOpenAPIBytes parses OpenAPI JSON bytes into the SDK IR.
func ParseOpenAPIBytes(data []byte) (*SDKDocument, error) {
	var doc openAPIDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing OpenAPI JSON: %w", err)
	}

	// Parse component schemas
	schemas := make(map[string]*SchemaNode)
	if doc.Components != nil {
		for name, raw := range doc.Components.Schemas {
			node, err := parseSchemaNode(raw)
			if err != nil {
				return nil, fmt.Errorf("parsing schema %q: %w", name, err)
			}
			node.Name = name
			schemas[name] = node
		}
	}

	resolver := &schemaResolver{schemas: schemas}
	resolver.initFingerprints()

	// version → controller name → *ControllerGroup
	type versionedCtrl struct {
		version string
		ctrl    string
	}
	groups := make(map[versionedCtrl]*ControllerGroup)

	for pathStr, pathItem := range doc.Paths {
		for method, rawOp := range pathItem {
			httpMethod := strings.ToUpper(method)
			if !isHTTPMethod(httpMethod) {
				continue
			}

			var op openAPIOperation
			if err := json.Unmarshal(rawOp, &op); err != nil {
				return nil, fmt.Errorf("parsing operation %s %s: %w", httpMethod, pathStr, err)
			}

			// Extract version from path prefix
			version := extractVersion(pathStr)

			// Determine controller name
			ctrlName := resolveControllerName(op, pathStr, version)

			// Determine method name
			methodName := resolveMethodName(op, httpMethod, pathStr)

			// Build SDKMethod
			sdkMethod := SDKMethod{
				Name:        methodName,
				HTTPMethod:  httpMethod,
				Path:        pathStr,
				Summary:     op.Summary,
				Description: op.Description,
				Deprecated:  op.Deprecated,
			}

			// Parse parameters
			for _, param := range op.Parameters {
				tsType := resolver.schemaToTS(param.Schema)
				sdkParam := SDKParam{
					Name:     param.Name,
					TSType:   tsType,
					Required: param.Required,
				}
				switch param.In {
				case "path":
					sdkParam.Required = true
					sdkMethod.PathParams = append(sdkMethod.PathParams, sdkParam)
				case "query":
					sdkMethod.QueryParams = append(sdkMethod.QueryParams, sdkParam)
				case "header":
					sdkMethod.HeaderParams = append(sdkMethod.HeaderParams, sdkParam)
				}
			}

			// Parse request body
			if op.RequestBody != nil {
				contentType := "application/json"
				var schemaRaw json.RawMessage
				// Priority: multipart/form-data > application/json > first available
				if ct, ok := op.RequestBody.Content["multipart/form-data"]; ok {
					contentType = "multipart/form-data"
					schemaRaw = ct.Schema
				} else if ct, ok := op.RequestBody.Content["application/json"]; ok {
					schemaRaw = ct.Schema
				} else {
					for ct, media := range op.RequestBody.Content {
						contentType = ct
						schemaRaw = media.Schema
						break
					}
				}
				tsType := resolver.schemaToTS(schemaRaw)
				sdkMethod.Body = &SDKBody{
					TSType:      tsType,
					Required:    op.RequestBody.Required,
					ContentType: contentType,
				}
			}

			// Parse response type
			resp := resolveResponse(op.Responses, resolver)
			sdkMethod.ResponseType = resp.tsType
			sdkMethod.ResponseStatus = resp.status
			sdkMethod.ResponseContentType = resp.contentType
			sdkMethod.IsVoid = resp.isVoid

			// Extract typed SSE event data from itemSchema
			if resp.contentType == "text/event-stream" {
				sdkMethod.SSEEventType = extractSSEEventType(op.Responses, resolver)
			}

			// Add to version+controller group
			key := versionedCtrl{version: version, ctrl: ctrlName}
			group, exists := groups[key]
			if !exists {
				group = &ControllerGroup{Name: ctrlName}
				groups[key] = group
			}
			group.Methods = append(group.Methods, sdkMethod)
		}
	}

	// Organize into VersionGroups
	versionMap := make(map[string][]ControllerGroup)
	for key, group := range groups {
		sort.Slice(group.Methods, func(i, j int) bool {
			return group.Methods[i].Name < group.Methods[j].Name
		})
		versionMap[key.version] = append(versionMap[key.version], *group)
	}

	var versions []VersionGroup
	for ver, ctrls := range versionMap {
		sort.Slice(ctrls, func(i, j int) bool {
			return ctrls[i].Name < ctrls[j].Name
		})
		versions = append(versions, VersionGroup{
			Version:     ver,
			Controllers: ctrls,
		})
	}
	// Sort: unversioned ("") first, then v1, v2, etc.
	sort.Slice(versions, func(i, j int) bool {
		if versions[i].Version == "" {
			return true
		}
		if versions[j].Version == "" {
			return false
		}
		return versions[i].Version < versions[j].Version
	})

	return &SDKDocument{
		Versions: versions,
		Schemas:  schemas,
	}, nil
}

// extractVersion extracts the version prefix from a path.
// e.g., "/v1/orders" → "v1", "/orders" → ""
func extractVersion(path string) string {
	if m := versionPrefixRe.FindStringSubmatch(path); m != nil {
		return "v" + m[1]
	}
	return ""
}

func resolveControllerName(op openAPIOperation, pathStr, version string) string {
	if op.XTsgonestController != "" {
		return op.XTsgonestController
	}
	if len(op.Tags) > 0 && op.Tags[0] != "" {
		return op.Tags[0] + "Controller"
	}
	// Fallback: first meaningful path segment (skip version prefix)
	trimmed := pathStr
	if version != "" {
		trimmed = versionPrefixRe.ReplaceAllString(pathStr, "/")
	}
	segments := strings.Split(strings.TrimPrefix(trimmed, "/"), "/")
	if len(segments) > 0 && segments[0] != "" {
		return capitalize(segments[0]) + "Controller"
	}
	return "DefaultController"
}

func resolveMethodName(op openAPIOperation, httpMethod, pathStr string) string {
	if op.XTsgonestMethod != "" {
		return op.XTsgonestMethod
	}
	if op.OperationID != "" {
		return op.OperationID
	}
	// Synthesize from method + path: GET /users/{id} → getUsers_id
	cleaned := strings.NewReplacer(
		"/", "_",
		"{", "",
		"}", "",
		"-", "_",
	).Replace(pathStr)
	cleaned = strings.Trim(cleaned, "_")
	return strings.ToLower(httpMethod) + capitalize(cleaned)
}

type resolvedResponse struct {
	tsType      string
	status      int
	contentType string
	isVoid      bool
}

func resolveResponseType(responses map[string]*openAPIResponse, resolver *schemaResolver) (tsType string, status int, isVoid bool) {
	r := resolveResponse(responses, resolver)
	return r.tsType, r.status, r.isVoid
}

func resolveResponse(responses map[string]*openAPIResponse, resolver *schemaResolver) resolvedResponse {
	priorities := []string{"200", "201", "202", "203", "204"}
	for _, code := range priorities {
		resp, ok := responses[code]
		if !ok {
			continue
		}
		status := statusCodeToInt(code)
		if code == "204" || resp.Content == nil || len(resp.Content) == 0 {
			return resolvedResponse{"void", status, "", true}
		}
		// Prefer application/json
		if ct, ok := resp.Content["application/json"]; ok {
			return resolvedResponse{resolver.schemaToTS(ct.Schema), status, "application/json", false}
		}
		// Map non-JSON content types to appropriate TS types
		for contentType, media := range resp.Content {
			tsType := contentTypeToTSType(contentType, media, resolver)
			return resolvedResponse{tsType, status, contentType, false}
		}
	}

	// Check any 2xx
	for code, resp := range responses {
		if !strings.HasPrefix(code, "2") {
			continue
		}
		status := statusCodeToInt(code)
		if resp.Content == nil || len(resp.Content) == 0 {
			return resolvedResponse{"void", status, "", true}
		}
		if ct, ok := resp.Content["application/json"]; ok {
			return resolvedResponse{resolver.schemaToTS(ct.Schema), status, "application/json", false}
		}
		for contentType, media := range resp.Content {
			tsType := contentTypeToTSType(contentType, media, resolver)
			return resolvedResponse{tsType, status, contentType, false}
		}
	}

	return resolvedResponse{"void", 200, "", true}
}

// contentTypeToTSType maps a response content type to an appropriate TypeScript type.
func contentTypeToTSType(contentType string, media openAPIMediaType, resolver *schemaResolver) string {
	switch {
	case contentType == "application/json":
		return resolver.schemaToTS(media.Schema)
	case contentType == "text/event-stream":
		return "ReadableStream<Uint8Array>"
	case contentType == "text/plain" || contentType == "text/html" || contentType == "text/csv":
		return "string"
	case strings.HasPrefix(contentType, "application/octet-stream"),
		strings.HasPrefix(contentType, "application/pdf"),
		strings.HasPrefix(contentType, "image/"),
		strings.HasPrefix(contentType, "audio/"),
		strings.HasPrefix(contentType, "video/"):
		return "Blob"
	case contentType == "multipart/form-data":
		return "FormData"
	default:
		// For unknown content types, try to parse schema if present
		if media.Schema != nil {
			return resolver.schemaToTS(media.Schema)
		}
		return "Blob"
	}
}

// extractSSEEventType extracts the typed event data type from a text/event-stream response.
// It looks at itemSchema.properties.data.contentSchema for a $ref or inline schema.
// Returns the TypeScript type name, or "" for untyped SSE.
func extractSSEEventType(responses map[string]*openAPIResponse, resolver *schemaResolver) string {
	// Find the response with text/event-stream content
	priorities := []string{"200", "201", "202", "203"}
	for _, code := range priorities {
		resp, ok := responses[code]
		if !ok {
			continue
		}
		if resp.Content == nil {
			continue
		}
		media, ok := resp.Content["text/event-stream"]
		if !ok {
			continue
		}
		return extractSSETypeFromMedia(media, resolver)
	}
	// Check any 2xx
	for code, resp := range responses {
		if !strings.HasPrefix(code, "2") || resp.Content == nil {
			continue
		}
		media, ok := resp.Content["text/event-stream"]
		if !ok {
			continue
		}
		return extractSSETypeFromMedia(media, resolver)
	}
	return ""
}

func extractSSETypeFromMedia(media openAPIMediaType, resolver *schemaResolver) string {
	if media.ItemSchema == nil {
		return ""
	}
	var itemSchema map[string]json.RawMessage
	if err := json.Unmarshal(media.ItemSchema, &itemSchema); err != nil {
		return ""
	}
	propsRaw, ok := itemSchema["properties"]
	if !ok {
		return ""
	}
	var props map[string]json.RawMessage
	if err := json.Unmarshal(propsRaw, &props); err != nil {
		return ""
	}
	dataRaw, ok := props["data"]
	if !ok {
		return ""
	}
	var dataProp map[string]json.RawMessage
	if err := json.Unmarshal(dataRaw, &dataProp); err != nil {
		return ""
	}
	contentSchemaRaw, ok := dataProp["contentSchema"]
	if !ok {
		return ""
	}
	return resolver.schemaToTS(contentSchemaRaw)
}

func statusCodeToInt(code string) int {
	n := 0
	for _, c := range code {
		n = n*10 + int(c-'0')
	}
	return n
}

// schemaResolver converts raw JSON schema references to TypeScript type strings.
type schemaResolver struct {
	schemas      map[string]*SchemaNode
	fingerprints map[string]string // property fingerprint → schema name (for inline matching)
}

// initFingerprints builds a lookup table of schema fingerprints for inline object matching.
func (r *schemaResolver) initFingerprints() {
	r.fingerprints = make(map[string]string)
	for name, node := range r.schemas {
		if node.Type == "object" && len(node.Properties) > 0 {
			fp := schemaFingerprint(node)
			r.fingerprints[fp] = name
		}
	}
}

// schemaFingerprint returns a string fingerprint of an object schema's property names and required set.
func schemaFingerprint(node *SchemaNode) string {
	var names []string
	for name := range node.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	reqSet := make(map[string]bool)
	for _, r := range node.Required {
		reqSet[r] = true
	}
	var parts []string
	for _, n := range names {
		marker := "?"
		if reqSet[n] {
			marker = "!"
		}
		parts = append(parts, n+marker)
	}
	return strings.Join(parts, ",")
}

// rawSchemaFingerprint extracts a fingerprint from raw JSON schema (for matching inline objects).
func rawSchemaFingerprint(node map[string]json.RawMessage) string {
	propsRaw, hasProps := node["properties"]
	if !hasProps {
		return ""
	}
	var props map[string]json.RawMessage
	json.Unmarshal(propsRaw, &props)

	var names []string
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)

	var requiredList []string
	if reqRaw, ok := node["required"]; ok {
		json.Unmarshal(reqRaw, &requiredList)
	}
	reqSet := make(map[string]bool)
	for _, r := range requiredList {
		reqSet[r] = true
	}

	var parts []string
	for _, n := range names {
		marker := "?"
		if reqSet[n] {
			marker = "!"
		}
		parts = append(parts, n+marker)
	}
	return strings.Join(parts, ",")
}

func (r *schemaResolver) schemaToTS(raw json.RawMessage) string {
	if raw == nil || len(raw) == 0 {
		return "unknown"
	}

	var node map[string]json.RawMessage
	if err := json.Unmarshal(raw, &node); err != nil {
		return "unknown"
	}

	// Handle $ref
	if refRaw, ok := node["$ref"]; ok {
		var ref string
		json.Unmarshal(refRaw, &ref)
		name := strings.TrimPrefix(ref, "#/components/schemas/")
		return name
	}

	// Handle type
	var typeStr string
	if t, ok := node["type"]; ok {
		json.Unmarshal(t, &typeStr)
	}

	// Handle enum
	if enumRaw, ok := node["enum"]; ok {
		var values []any
		json.Unmarshal(enumRaw, &values)
		return enumToTSUnion(values)
	}

	// Handle const
	if constRaw, ok := node["const"]; ok {
		var val any
		json.Unmarshal(constRaw, &val)
		return constToTS(val)
	}

	// Handle anyOf
	if anyOfRaw, ok := node["anyOf"]; ok {
		return r.compositeToTS(anyOfRaw, " | ")
	}

	// Handle oneOf
	if oneOfRaw, ok := node["oneOf"]; ok {
		return r.compositeToTS(oneOfRaw, " | ")
	}

	// Handle allOf
	if allOfRaw, ok := node["allOf"]; ok {
		return r.compositeToTS(allOfRaw, " & ")
	}

	// Handle format: binary → Blob (file upload/download)
	var formatStr string
	if f, ok := node["format"]; ok {
		json.Unmarshal(f, &formatStr)
	}

	switch typeStr {
	case "string":
		if formatStr == "binary" {
			return "Blob"
		}
		return "string"
	case "number", "integer":
		return "number"
	case "boolean":
		return "boolean"
	case "null":
		return "null"
	case "array":
		if itemsRaw, ok := node["items"]; ok {
			itemType := r.schemaToTS(itemsRaw)
			if strings.Contains(itemType, " | ") || strings.Contains(itemType, " & ") {
				return "(" + itemType + ")[]"
			}
			return itemType + "[]"
		}
		return "unknown[]"
	case "object":
		if addPropsRaw, ok := node["additionalProperties"]; ok {
			valType := r.schemaToTS(addPropsRaw)
			return "Record<string, " + valType + ">"
		}
		// Try to match inline object against known component schemas
		if r.fingerprints != nil {
			fp := rawSchemaFingerprint(node)
			if fp != "" {
				if name, ok := r.fingerprints[fp]; ok {
					return name
				}
			}
		}
		return r.inlineObjectToTS(node)
	default:
		return "unknown"
	}
}

func (r *schemaResolver) compositeToTS(raw json.RawMessage, sep string) string {
	var items []json.RawMessage
	json.Unmarshal(raw, &items)

	var parts []string
	hasNull := false
	for _, item := range items {
		ts := r.schemaToTS(item)
		if ts == "null" {
			hasNull = true
			continue
		}
		parts = append(parts, ts)
	}

	result := strings.Join(parts, sep)
	if len(parts) > 1 {
		result = "(" + result + ")"
	}
	if hasNull {
		result += " | null"
	}
	return result
}

func (r *schemaResolver) inlineObjectToTS(node map[string]json.RawMessage) string {
	propsRaw, hasProps := node["properties"]
	if !hasProps {
		return "Record<string, unknown>"
	}

	var props map[string]json.RawMessage
	json.Unmarshal(propsRaw, &props)

	var requiredList []string
	if reqRaw, ok := node["required"]; ok {
		json.Unmarshal(reqRaw, &requiredList)
	}
	requiredSet := make(map[string]bool)
	for _, r := range requiredList {
		requiredSet[r] = true
	}

	var names []string
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)

	var fields []string
	for _, name := range names {
		tsType := r.schemaToTS(props[name])
		opt := "?"
		if requiredSet[name] {
			opt = ""
		}
		fields = append(fields, fmt.Sprintf("%s%s: %s", name, opt, tsType))
	}

	return "{ " + strings.Join(fields, "; ") + " }"
}

func enumToTSUnion(values []any) string {
	var parts []string
	for _, v := range values {
		parts = append(parts, constToTS(v))
	}
	return strings.Join(parts, " | ")
}

func constToTS(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func isHTTPMethod(m string) bool {
	switch m {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return true
	}
	return false
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parseSchemaNode parses a raw JSON schema into a SchemaNode.
func parseSchemaNode(raw json.RawMessage) (*SchemaNode, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	node := &SchemaNode{}

	if v, ok := m["$ref"]; ok {
		var ref string
		json.Unmarshal(v, &ref)
		node.Ref = strings.TrimPrefix(ref, "#/components/schemas/")
		return node, nil
	}

	if v, ok := m["type"]; ok {
		json.Unmarshal(v, &node.Type)
	}
	if v, ok := m["format"]; ok {
		json.Unmarshal(v, &node.Format)
	}
	if v, ok := m["description"]; ok {
		json.Unmarshal(v, &node.Description)
	}

	if v, ok := m["enum"]; ok {
		json.Unmarshal(v, &node.Enum)
	}
	if v, ok := m["const"]; ok {
		json.Unmarshal(v, &node.Const)
	}

	if v, ok := m["properties"]; ok {
		var props map[string]json.RawMessage
		json.Unmarshal(v, &props)
		node.Properties = make(map[string]*SchemaNode)
		for name, propRaw := range props {
			propNode, err := parseSchemaNode(propRaw)
			if err != nil {
				return nil, fmt.Errorf("property %q: %w", name, err)
			}
			node.Properties[name] = propNode
		}
	}
	if v, ok := m["required"]; ok {
		json.Unmarshal(v, &node.Required)
	}

	if v, ok := m["items"]; ok {
		itemNode, err := parseSchemaNode(v)
		if err != nil {
			return nil, fmt.Errorf("items: %w", err)
		}
		node.Items = itemNode
	}

	if v, ok := m["additionalProperties"]; ok {
		var boolVal bool
		if json.Unmarshal(v, &boolVal) == nil {
			// boolean — ignore
		} else {
			addNode, err := parseSchemaNode(v)
			if err == nil {
				node.AdditionalProperties = addNode
			}
		}
	}

	if v, ok := m["anyOf"]; ok {
		node.AnyOf = parseSchemaArray(v)
	}
	if v, ok := m["oneOf"]; ok {
		node.OneOf = parseSchemaArray(v)
	}
	if v, ok := m["allOf"]; ok {
		node.AllOf = parseSchemaArray(v)
	}

	if v, ok := m["discriminator"]; ok {
		var disc Discriminator
		json.Unmarshal(v, &disc)
		node.Discriminator = &disc
	}

	return node, nil
}

func parseSchemaArray(raw json.RawMessage) []*SchemaNode {
	var items []json.RawMessage
	json.Unmarshal(raw, &items)
	var nodes []*SchemaNode
	for _, item := range items {
		node, err := parseSchemaNode(item)
		if err == nil {
			nodes = append(nodes, node)
		}
	}
	return nodes
}
