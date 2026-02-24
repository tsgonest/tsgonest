package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// Golden tests verify the structural shape of generated OpenAPI documents.
// These use inline comparisons rather than golden files since the tests
// run inside the openapi package and need to be self-contained.

// buildSimpleControllerDoc generates a standard CRUD controller document
// used across multiple golden tests.
func buildSimpleControllerDoc() *Document {
	registry := metadata.NewTypeRegistry()
	registry.Register("CreateUserDto", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "CreateUserDto",
		Properties: []metadata.Property{
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})
	registry.Register("UserResponse", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "UserResponse",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "email", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: false},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/users",
					OperationID: "findAll",
					ReturnType:  metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"}},
					StatusCode:  200,
					Tags:        []string{"User"},
				},
				{
					Method:      "GET",
					Path:        "/users/:id",
					OperationID: "findOne",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
					StatusCode: 200,
					Tags:       []string{"User"},
				},
				{
					Method:      "POST",
					Path:        "/users",
					OperationID: "create",
					Parameters: []analyzer.RouteParameter{
						{Category: "body", Name: "", Type: metadata.Metadata{Kind: metadata.KindRef, Ref: "CreateUserDto"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "UserResponse"},
					StatusCode: 201,
					Tags:       []string{"User"},
				},
				{
					Method:      "DELETE",
					Path:        "/users/:id",
					OperationID: "remove",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindVoid},
					StatusCode: 204,
					Tags:       []string{"User"},
				},
			},
		},
	}

	return gen.Generate(controllers)
}

func TestGolden_SimpleController_Structure(t *testing.T) {
	doc := buildSimpleControllerDoc()

	jsonBytes, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Parse back as generic map to verify structure
	var raw map[string]any
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Top-level required fields
	if raw["openapi"] != "3.2.0" {
		t.Errorf("expected openapi=3.2.0, got %v", raw["openapi"])
	}

	info, ok := raw["info"].(map[string]any)
	if !ok {
		t.Fatal("expected info object")
	}
	if info["title"] != "API" {
		t.Errorf("expected info.title=API, got %v", info["title"])
	}
	if info["version"] != "1.0.0" {
		t.Errorf("expected info.version=1.0.0, got %v", info["version"])
	}

	// Paths
	paths, ok := raw["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected paths object")
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
	if _, ok := paths["/users"]; !ok {
		t.Error("expected /users path")
	}
	if _, ok := paths["/users/{id}"]; !ok {
		t.Error("expected /users/{id} path")
	}

	// Components
	components, ok := raw["components"].(map[string]any)
	if !ok {
		t.Fatal("expected components object")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("expected components.schemas object")
	}
	if _, ok := schemas["UserResponse"]; !ok {
		t.Error("expected UserResponse schema")
	}
	if _, ok := schemas["CreateUserDto"]; !ok {
		t.Error("expected CreateUserDto schema")
	}

	// Tags
	tags, ok := raw["tags"].([]any)
	if !ok {
		t.Fatal("expected tags array")
	}
	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tags))
	}

	// Verify /users has GET and POST
	usersPath, ok := paths["/users"].(map[string]any)
	if !ok {
		t.Fatal("expected /users to be an object")
	}
	if _, ok := usersPath["get"]; !ok {
		t.Error("expected GET on /users")
	}
	if _, ok := usersPath["post"]; !ok {
		t.Error("expected POST on /users")
	}

	// Verify /users/{id} has GET and DELETE
	usersIdPath, ok := paths["/users/{id}"].(map[string]any)
	if !ok {
		t.Fatal("expected /users/{id} to be an object")
	}
	if _, ok := usersIdPath["get"]; !ok {
		t.Error("expected GET on /users/{id}")
	}
	if _, ok := usersIdPath["delete"]; !ok {
		t.Error("expected DELETE on /users/{id}")
	}
}

func TestGolden_SimpleController_CorrectStatusCodes(t *testing.T) {
	doc := buildSimpleControllerDoc()

	jsonBytes, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	paths := raw["paths"].(map[string]any)

	// GET /users → 200
	usersPath := paths["/users"].(map[string]any)
	getOp := usersPath["get"].(map[string]any)
	getResps := getOp["responses"].(map[string]any)
	if _, ok := getResps["200"]; !ok {
		t.Error("expected 200 response for GET /users")
	}

	// POST /users → 201
	postOp := usersPath["post"].(map[string]any)
	postResps := postOp["responses"].(map[string]any)
	if _, ok := postResps["201"]; !ok {
		t.Error("expected 201 response for POST /users")
	}

	// DELETE /users/{id} → 204
	usersIdPath := paths["/users/{id}"].(map[string]any)
	deleteOp := usersIdPath["delete"].(map[string]any)
	deleteResps := deleteOp["responses"].(map[string]any)
	if _, ok := deleteResps["204"]; !ok {
		t.Error("expected 204 response for DELETE /users/{id}")
	}

	// All status codes should be valid string keys
	validCodes := map[string]bool{"200": true, "201": true, "204": true}
	allPaths := []map[string]any{usersPath, usersIdPath}
	for _, p := range allPaths {
		for _, methodVal := range p {
			method, ok := methodVal.(map[string]any)
			if !ok {
				continue
			}
			resps, ok := method["responses"].(map[string]any)
			if !ok {
				continue
			}
			for code := range resps {
				if !validCodes[code] {
					t.Errorf("unexpected status code %q in responses", code)
				}
			}
		}
	}
}

func TestGolden_SimpleController_RequiredFields(t *testing.T) {
	doc := buildSimpleControllerDoc()

	jsonBytes, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Required top-level keys
	requiredKeys := []string{`"openapi"`, `"info"`, `"paths"`}
	for _, key := range requiredKeys {
		if !strings.Contains(jsonStr, key) {
			t.Errorf("expected JSON to contain %s", key)
		}
	}

	// Required info fields
	infoKeys := []string{`"title"`, `"version"`}
	for _, key := range infoKeys {
		if !strings.Contains(jsonStr, key) {
			t.Errorf("expected JSON to contain info field %s", key)
		}
	}

	// All operations should have responses
	var raw map[string]any
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	paths := raw["paths"].(map[string]any)
	for pathStr, pathVal := range paths {
		pathObj := pathVal.(map[string]any)
		for method, opVal := range pathObj {
			op, ok := opVal.(map[string]any)
			if !ok {
				continue
			}
			if _, hasResponses := op["responses"]; !hasResponses {
				t.Errorf("operation %s %s missing responses", strings.ToUpper(method), pathStr)
			}
		}
	}

	// Component schemas should have 'type' or '$ref'
	components := raw["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	for name, schemaVal := range schemas {
		schema := schemaVal.(map[string]any)
		_, hasType := schema["type"]
		_, hasRef := schema["$ref"]
		if !hasType && !hasRef {
			t.Errorf("schema %q missing both 'type' and '$ref'", name)
		}
	}
}

func TestGolden_SimpleController_ResponseDescriptions(t *testing.T) {
	doc := buildSimpleControllerDoc()

	// All responses should have descriptions (OAS 3.1 requirement)
	for path, item := range doc.Paths {
		ops := map[string]*Operation{
			"GET":    item.Get,
			"POST":   item.Post,
			"DELETE": item.Delete,
		}
		for method, op := range ops {
			if op == nil {
				continue
			}
			for code, resp := range op.Responses {
				if resp.Description == "" {
					t.Errorf("%s %s response %s missing description", method, path, code)
				}
			}
		}
	}
}

func TestGolden_SimpleController_OperationIDs(t *testing.T) {
	doc := buildSimpleControllerDoc()

	// Collect all operation IDs and verify uniqueness
	opIDs := make(map[string]string) // operationID → "METHOD path"
	for path, item := range doc.Paths {
		ops := map[string]*Operation{
			"GET":    item.Get,
			"POST":   item.Post,
			"PUT":    item.Put,
			"DELETE": item.Delete,
			"PATCH":  item.Patch,
		}
		for method, op := range ops {
			if op == nil {
				continue
			}
			if op.OperationID == "" {
				continue // operationID is optional in OAS 3.1
			}
			key := method + " " + path
			if existing, dup := opIDs[op.OperationID]; dup {
				t.Errorf("duplicate operationId %q: %s and %s", op.OperationID, existing, key)
			}
			opIDs[op.OperationID] = key
		}
	}

	// Verify expected operation IDs are present
	expectedOps := []string{"findAll", "findOne", "create", "remove"}
	for _, expected := range expectedOps {
		if _, ok := opIDs[expected]; !ok {
			t.Errorf("expected operationId %q not found", expected)
		}
	}
}

func TestGolden_SimpleController_PathParameters(t *testing.T) {
	doc := buildSimpleControllerDoc()

	// Verify /users/{id} GET has a path parameter
	idPath, ok := doc.Paths["/users/{id}"]
	if !ok {
		t.Fatal("expected /users/{id} path")
	}

	if idPath.Get == nil {
		t.Fatal("expected GET on /users/{id}")
	}

	if len(idPath.Get.Parameters) != 1 {
		t.Fatalf("expected 1 parameter on GET /users/{id}, got %d", len(idPath.Get.Parameters))
	}

	param := idPath.Get.Parameters[0]
	if param.Name != "id" {
		t.Errorf("expected param name=id, got %q", param.Name)
	}
	if param.In != "path" {
		t.Errorf("expected param in=path, got %q", param.In)
	}
	if !param.Required {
		t.Error("expected path param to be required")
	}
}

func TestGolden_SimpleController_RequestBody(t *testing.T) {
	doc := buildSimpleControllerDoc()

	usersPath, ok := doc.Paths["/users"]
	if !ok || usersPath.Post == nil {
		t.Fatal("expected POST /users")
	}

	rb := usersPath.Post.RequestBody
	if rb == nil {
		t.Fatal("expected request body on POST /users")
	}
	if !rb.Required {
		t.Error("expected request body to be required")
	}

	jsonContent, ok := rb.Content["application/json"]
	if !ok {
		t.Fatal("expected application/json content")
	}
	if jsonContent.Schema == nil {
		t.Fatal("expected schema in request body")
	}
	if jsonContent.Schema.Ref != "#/components/schemas/CreateUserDto" {
		t.Errorf("expected $ref to CreateUserDto, got %q", jsonContent.Schema.Ref)
	}
}

func TestGolden_SimpleController_ComponentSchemas(t *testing.T) {
	doc := buildSimpleControllerDoc()

	if doc.Components == nil || doc.Components.Schemas == nil {
		t.Fatal("expected components.schemas")
	}

	// Verify UserResponse schema shape
	userResp, ok := doc.Components.Schemas["UserResponse"]
	if !ok {
		t.Fatal("expected UserResponse schema")
	}
	if userResp.Type != "object" {
		t.Errorf("expected UserResponse type=object, got %q", userResp.Type)
	}
	if len(userResp.Properties) != 3 {
		t.Errorf("expected 3 properties in UserResponse, got %d", len(userResp.Properties))
	}
	if len(userResp.Required) != 2 {
		t.Errorf("expected 2 required fields in UserResponse, got %d", len(userResp.Required))
	}

	// Verify CreateUserDto schema shape
	createDto, ok := doc.Components.Schemas["CreateUserDto"]
	if !ok {
		t.Fatal("expected CreateUserDto schema")
	}
	if createDto.Type != "object" {
		t.Errorf("expected CreateUserDto type=object, got %q", createDto.Type)
	}
	if len(createDto.Properties) != 2 {
		t.Errorf("expected 2 properties in CreateUserDto, got %d", len(createDto.Properties))
	}
	if len(createDto.Required) != 2 {
		t.Errorf("expected 2 required fields in CreateUserDto, got %d", len(createDto.Required))
	}
}

func TestGolden_MultiController_Structure(t *testing.T) {
	registry := metadata.NewTypeRegistry()
	registry.Register("User", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "User",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "name", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})
	registry.Register("Post", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Post",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "title", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "body", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
		},
	})
	registry.Register("Comment", &metadata.Metadata{
		Kind: metadata.KindObject,
		Name: "Comment",
		Properties: []metadata.Property{
			{Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
			{Name: "text", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
			{Name: "postId", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"}, Required: true},
		},
	})

	gen := NewGenerator(registry)
	controllers := []analyzer.ControllerInfo{
		{
			Name: "UserController",
			Path: "users",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/users",
					OperationID: "listUsers",
					ReturnType:  metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "User"}},
					StatusCode:  200,
					Tags:        []string{"User"},
				},
				{
					Method:      "GET",
					Path:        "/users/:id",
					OperationID: "getUser",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "id", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindRef, Ref: "User"},
					StatusCode: 200,
					Tags:       []string{"User"},
				},
			},
		},
		{
			Name: "PostController",
			Path: "posts",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/posts",
					OperationID: "listPosts",
					ReturnType:  metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "Post"}},
					StatusCode:  200,
					Tags:        []string{"Post"},
				},
			},
		},
		{
			Name: "CommentController",
			Path: "comments",
			Routes: []analyzer.Route{
				{
					Method:      "GET",
					Path:        "/posts/:postId/comments",
					OperationID: "listComments",
					Parameters: []analyzer.RouteParameter{
						{Category: "param", Name: "postId", Type: metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"}, Required: true},
					},
					ReturnType: metadata.Metadata{Kind: metadata.KindArray, ElementType: &metadata.Metadata{Kind: metadata.KindRef, Ref: "Comment"}},
					StatusCode: 200,
					Tags:       []string{"Comment"},
				},
			},
		},
	}

	doc := gen.Generate(controllers)

	// Verify paths from all controllers
	expectedPaths := []string{"/users", "/users/{id}", "/posts", "/posts/{postId}/comments"}
	for _, p := range expectedPaths {
		if _, ok := doc.Paths[p]; !ok {
			t.Errorf("expected path %q not found", p)
		}
	}

	// Verify tags from all controllers
	tagNames := make(map[string]bool)
	for _, tag := range doc.Tags {
		tagNames[tag.Name] = true
	}
	expectedTags := []string{"User", "Post", "Comment"}
	for _, tag := range expectedTags {
		if !tagNames[tag] {
			t.Errorf("expected tag %q not found", tag)
		}
	}

	// Verify all schemas registered
	if doc.Components == nil || doc.Components.Schemas == nil {
		t.Fatal("expected components.schemas")
	}
	expectedSchemas := []string{"User", "Post", "Comment"}
	for _, s := range expectedSchemas {
		if _, ok := doc.Components.Schemas[s]; !ok {
			t.Errorf("expected schema %q not found", s)
		}
	}

	// Validate the multi-controller doc
	errors := ValidateDocument(doc)
	if len(errors) != 0 {
		t.Errorf("multi-controller doc should pass validation, got %d errors:", len(errors))
		for _, e := range errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

func TestGolden_SimpleController_TsgonestExtensions(t *testing.T) {
	doc := buildSimpleControllerDoc()

	jsonBytes, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	paths := raw["paths"].(map[string]any)

	// GET /users should have x-tsgonest-controller and x-tsgonest-method
	usersPath := paths["/users"].(map[string]any)
	getOp := usersPath["get"].(map[string]any)
	if getOp["x-tsgonest-controller"] != "UserController" {
		t.Errorf("expected x-tsgonest-controller=UserController, got %v", getOp["x-tsgonest-controller"])
	}
	if getOp["x-tsgonest-method"] != "findAll" {
		t.Errorf("expected x-tsgonest-method=findAll, got %v", getOp["x-tsgonest-method"])
	}

	// POST /users
	postOp := usersPath["post"].(map[string]any)
	if postOp["x-tsgonest-controller"] != "UserController" {
		t.Errorf("expected x-tsgonest-controller=UserController, got %v", postOp["x-tsgonest-controller"])
	}
	if postOp["x-tsgonest-method"] != "create" {
		t.Errorf("expected x-tsgonest-method=create, got %v", postOp["x-tsgonest-method"])
	}

	// DELETE /users/{id}
	usersIdPath := paths["/users/{id}"].(map[string]any)
	deleteOp := usersIdPath["delete"].(map[string]any)
	if deleteOp["x-tsgonest-controller"] != "UserController" {
		t.Errorf("expected x-tsgonest-controller=UserController, got %v", deleteOp["x-tsgonest-controller"])
	}
	if deleteOp["x-tsgonest-method"] != "remove" {
		t.Errorf("expected x-tsgonest-method=remove, got %v", deleteOp["x-tsgonest-method"])
	}
}

func TestGolden_MultiController_JSONRoundTrip(t *testing.T) {
	doc := buildSimpleControllerDoc()

	// Serialize to JSON
	jsonBytes, err := doc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Deserialize back
	var roundTripped Document
	if err := json.Unmarshal(jsonBytes, &roundTripped); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Re-serialize
	jsonBytes2, err := roundTripped.ToJSON()
	if err != nil {
		t.Fatalf("second ToJSON failed: %v", err)
	}

	// The JSON should be identical (stable serialization)
	if string(jsonBytes) != string(jsonBytes2) {
		t.Error("JSON round-trip produced different output")
		t.Logf("First:  %s", jsonBytes[:min(200, len(jsonBytes))])
		t.Logf("Second: %s", jsonBytes2[:min(200, len(jsonBytes2))])
	}
}
