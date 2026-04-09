package sdkgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate_BasicFixture(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/basic.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Verify expected files exist
	expectedFiles := []string{
		"types.ts",
		"client.ts",
		"sse.ts",
		"form-data.ts",
		"index.ts",
		"orders/index.ts",
		"products/index.ts",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %q to exist", f)
		}
	}

	// Verify types.ts contains schema interfaces
	typesContent := readFile(t, filepath.Join(outputDir, "types.ts"))
	assertContains(t, typesContent, "export interface Order {", "types.ts should contain Order interface")
	assertContains(t, typesContent, "export interface CreateOrderDto {", "types.ts should contain CreateOrderDto interface")
	assertContains(t, typesContent, "export interface Product {", "types.ts should contain Product interface")
	assertContains(t, typesContent, "export interface OrderItem {", "types.ts should contain OrderItem interface")

	// Verify client.ts contains core infrastructure
	clientContent := readFile(t, filepath.Join(outputDir, "client.ts"))
	assertContains(t, clientContent, "export interface SDKError", "client.ts should contain SDKError")
	assertContains(t, clientContent, "export type SDKResult", "client.ts should contain SDKResult")
	assertContains(t, clientContent, "export function createRequestFn", "client.ts should contain createRequestFn")
	assertContains(t, clientContent, "export interface RequestContext", "client.ts should contain RequestContext")
	assertContains(t, clientContent, "export function extractErrorMessage", "client.ts should contain extractErrorMessage")
	assertContains(t, clientContent, "timeout?: number", "client.ts should have timeout field")
	assertContains(t, clientContent, "throwOnError?: boolean", "client.ts should have throwOnError field")
	assertContains(t, clientContent, "onResponse?:", "client.ts should have onResponse hook")
	assertContains(t, clientContent, "onError?:", "client.ts should have onError hook")

	// Verify sse.ts contains SSEConnection
	sseContent := readFile(t, filepath.Join(outputDir, "sse.ts"))
	assertContains(t, sseContent, "export class SSEConnection", "sse.ts should contain SSEConnection")
	assertContains(t, sseContent, "export interface SSEEvent", "sse.ts should contain SSEEvent")

	// Verify form-data.ts contains buildFormData
	formDataContent := readFile(t, filepath.Join(outputDir, "form-data.ts"))
	assertContains(t, formDataContent, "export function buildFormData", "form-data.ts should contain buildFormData")

	// Verify types.ts contains JSDoc comments from schema descriptions
	assertContains(t, typesContent, "/** Represents a customer order */", "types.ts should have Order JSDoc")
	assertContains(t, typesContent, "/** Unique order identifier */", "types.ts should have id property JSDoc")
	assertContains(t, typesContent, "/** Current order status */", "types.ts should have status property JSDoc")

	// Verify orders controller
	ordersContent := readFile(t, filepath.Join(outputDir, "orders/index.ts"))
	assertContains(t, ordersContent, "export interface OrdersController", "should have OrdersController interface")
	assertContains(t, ordersContent, "export function createOrdersController", "should have factory")
	// Verify standalone exported functions (qualified with controller name)
	assertContains(t, ordersContent, "export async function Orders_listOrders(", "should have standalone Orders_listOrders")
	assertContains(t, ordersContent, "export async function Orders_createOrder(", "should have standalone Orders_createOrder")
	assertContains(t, ordersContent, "export async function Orders_getOrder(", "should have standalone Orders_getOrder")
	assertContains(t, ordersContent, "export async function Orders_deleteOrder(", "should have standalone Orders_deleteOrder")
	// Verify factory delegates to standalone functions with short keys
	assertContains(t, ordersContent, "listOrders: (options", "factory should delegate listOrders")
	assertContains(t, ordersContent, "=> Orders_listOrders(request", "factory should call standalone Orders_listOrders")
	assertContains(t, ordersContent, "import type {", "should import types")
	assertContains(t, ordersContent, "../types", "should reference ../types")
	assertContains(t, ordersContent, "../client", "should reference ../client")
	// Verify JSDoc on standalone functions
	assertContains(t, ordersContent, "* List all orders", "should have listOrders JSDoc summary")
	assertContains(t, ordersContent, "* Returns a paginated list of orders", "should have listOrders JSDoc description")

	// Verify index.ts re-exports
	indexContent := readFile(t, filepath.Join(outputDir, "index.ts"))
	assertContains(t, indexContent, "createClient", "index.ts should export createClient")
	assertContains(t, indexContent, "createOrdersController", "index.ts should export orders factory")
	assertContains(t, indexContent, "createProductsController", "index.ts should export products factory")
	// Verify standalone functions are re-exported with qualified names
	assertContains(t, indexContent, "Orders_listOrders", "index.ts should re-export standalone Orders_listOrders")
	// Verify RequestContext is re-exported
	assertContains(t, indexContent, "RequestContext", "index.ts should re-export RequestContext")
}

func TestGenerate_VersionedFixture(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/versioned.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Verify versioned directory structure
	expectedFiles := []string{
		"types.ts",
		"client.ts",
		"sse.ts",
		"form-data.ts",
		"index.ts",
		"health/index.ts",    // unversioned
		"v1/orders/index.ts", // versioned
		"v2/orders/index.ts", // versioned
	}
	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %q to exist", f)
		}
	}

	// Verify v1 controller uses ../../types (two levels up)
	v1Content := readFile(t, filepath.Join(outputDir, "v1/orders/index.ts"))
	assertContains(t, v1Content, "../../types", "v1 controller should reference ../../types")
	assertContains(t, v1Content, "../../client", "v1 controller should reference ../../client")

	// Verify unversioned controller uses ../types (one level up)
	healthContent := readFile(t, filepath.Join(outputDir, "health/index.ts"))
	assertContains(t, healthContent, "../client", "unversioned controller should reference ../client")

	// Verify index.ts has versioned structure
	indexContent := readFile(t, filepath.Join(outputDir, "index.ts"))
	assertContains(t, indexContent, "v1:", "index.ts should have v1 namespace")
	assertContains(t, indexContent, "v2:", "index.ts should have v2 namespace")

	// OrdersController exists in both v1 and v2 — should NOT be re-exported (name collision)
	assertNotContains(t, indexContent, "export { createOrdersController }", "colliding versioned controller should not be re-exported")

	// HealthController is unversioned — should still be re-exported
	assertContains(t, indexContent, "createHealthController", "unversioned controller should be re-exported")
}

func TestGenerate_SingleVersionFixture(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/single-version.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Verify versioned directory structure
	expectedFiles := []string{
		"types.ts",
		"client.ts",
		"index.ts",
		"v1/companies/index.ts",
		"v1/orders/index.ts",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %q to exist", f)
		}
	}

	// Verify index.ts re-exports versioned controllers (no name collisions)
	indexContent := readFile(t, filepath.Join(outputDir, "index.ts"))
	assertContains(t, indexContent, "createCompaniesController", "index.ts should re-export createCompaniesController")
	assertContains(t, indexContent, "createOrdersController", "index.ts should re-export createOrdersController")
	assertContains(t, indexContent, "from './v1/companies'", "should import from versioned path")
	assertContains(t, indexContent, "from './v1/orders'", "should import from versioned path")

	// Verify standalone functions are re-exported
	assertContains(t, indexContent, "Companies_listCompanies", "index.ts should re-export standalone Companies_listCompanies")
	assertContains(t, indexContent, "Orders_listOrders", "index.ts should re-export standalone Orders_listOrders")

	// Verify createClient still works
	assertContains(t, indexContent, "createClient", "index.ts should export createClient")
}

func TestGenerate_NoExtensions(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/no-extensions.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Should still generate valid output using fallback grouping
	expectedFiles := []string{
		"types.ts",
		"client.ts",
		"index.ts",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %q to exist", f)
		}
	}

	// Verify types.ts has schemas
	typesContent := readFile(t, filepath.Join(outputDir, "types.ts"))
	assertContains(t, typesContent, "User", "types.ts should contain User")
	assertContains(t, typesContent, "Post", "types.ts should contain Post")
}

func TestGenerate_ComplexTypes(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/complex-types.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	typesContent := readFile(t, filepath.Join(outputDir, "types.ts"))

	// Check discriminated union
	assertContains(t, typesContent, "ItemType", "should have ItemType")

	// Check enum
	assertContains(t, typesContent, "ItemStatus", "should have ItemStatus")

	// Check Record type
	assertContains(t, typesContent, "Metadata", "should have Metadata")

	// Check nullable
	assertContains(t, typesContent, "NullableField", "should have NullableField")
}

func TestGenerate_FileUploads(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/file-uploads.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Verify expected files exist
	expectedFiles := []string{
		"types.ts",
		"client.ts",
		"index.ts",
		"uploads/index.ts",
		"reports/index.ts",
		"events/index.ts",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %q to exist", f)
		}
	}

	uploadsContent := readFile(t, filepath.Join(outputDir, "uploads/index.ts"))

	// --- Single file upload ---
	assertContains(t, uploadsContent, "file: File | Blob", "single file field should be typed as File | Blob")

	// --- Array of files ((File | Blob)[]) ---
	assertContains(t, uploadsContent, "photos: (File | Blob)[]", "array file field should be typed as (File | Blob)[]")

	// --- Multiple file fields in one request ---
	assertContains(t, uploadsContent, "document: File | Blob", "document field should be File | Blob")
	assertContains(t, uploadsContent, "coverImage?: File | Blob", "optional coverImage field should be File | Blob")

	// --- Mixed file + text fields ---
	assertContains(t, uploadsContent, "albumName: string", "text field in multipart should be string")
	assertContains(t, uploadsContent, "title: string", "text field in multipart should be string")

	// --- buildFormData import for multipart controllers (from form-data.ts) ---
	assertContains(t, uploadsContent, "import { buildFormData }", "should import buildFormData")
	assertContains(t, uploadsContent, "from '../form-data'", "should import buildFormData from form-data.ts")
	assertContains(t, uploadsContent, "buildFormData(options.body)", "should call buildFormData")

	// --- FormData escape hatch: instanceof check ---
	assertContains(t, uploadsContent, "options.body instanceof FormData", "should have FormData escape hatch")

	// --- body type should include FormData union ---
	assertContains(t, uploadsContent, "| FormData", "body type should include FormData union")

	// --- upload progress option ---
	assertContains(t, uploadsContent, "onUploadProgress?:", "should have optional onUploadProgress callback")

	// --- contentType with uploadContentType fallback ---
	assertContains(t, uploadsContent, "uploadContentType ?? 'multipart/form-data'", "should set multipart content type with uploadContentType fallback")

	// --- JSDoc on upload methods ---
	assertContains(t, uploadsContent, "Upload a user avatar", "should have uploadAvatar JSDoc")
	assertContains(t, uploadsContent, "Upload multiple gallery photos", "should have uploadGallery JSDoc")

	// --- Non-JSON responses: PDF (Blob) ---
	reportsContent := readFile(t, filepath.Join(outputDir, "reports/index.ts"))
	assertContains(t, reportsContent, "SDKResult<Blob>", "PDF response should be Blob")
	assertContains(t, reportsContent, "responseType: 'blob'", "PDF should have blob responseType")

	// --- Non-JSON responses: CSV (text) ---
	assertContains(t, reportsContent, "SDKResult<string>", "CSV response should be string")
	assertContains(t, reportsContent, "responseType: 'text'", "CSV should have text responseType")

	// --- Typed SSE: SSEConnection<StreamEvent> ---
	eventsContent := readFile(t, filepath.Join(outputDir, "events/index.ts"))
	assertContains(t, eventsContent, "SSEConnection<StreamEvent>", "typed SSE response should be SSEConnection<StreamEvent>")
	assertContains(t, eventsContent, "responseType: 'sse'", "typed SSE should have sse responseType")
	assertContains(t, eventsContent, "import { SSEConnection }", "should import SSEConnection")
	assertContains(t, eventsContent, "from '../sse'", "should import SSEConnection from sse.ts")
	assertContains(t, eventsContent, "import type { StreamEvent }", "should import StreamEvent type")

	// --- Untyped SSE: SSEConnection<string> ---
	assertContains(t, eventsContent, "SSEConnection<string>", "untyped SSE response should be SSEConnection<string>")
	assertContains(t, eventsContent, "responseType: 'sse-raw'", "untyped SSE should have sse-raw responseType")

	// --- sse.ts should have SSEConnection class (split from client.ts) ---
	sseContentFU := readFile(t, filepath.Join(outputDir, "sse.ts"))
	assertContains(t, sseContentFU, "export class SSEConnection", "sse.ts should export SSEConnection class")
	assertContains(t, sseContentFU, "export interface SSEEvent", "sse.ts should export SSEEvent interface")

	// --- form-data.ts should have buildFormData utility (split from client.ts) ---
	formDataContentFU := readFile(t, filepath.Join(outputDir, "form-data.ts"))
	assertContains(t, formDataContentFU, "export function buildFormData", "form-data.ts should export buildFormData")
	assertContains(t, formDataContentFU, "instanceof File", "buildFormData should check File before Blob")
	assertContains(t, formDataContentFU, "instanceof Blob", "buildFormData should handle Blob")
	assertContains(t, formDataContentFU, "Array.isArray(value)", "buildFormData should handle arrays")
	assertContains(t, formDataContentFU, "FormDataCompatible", "form-data.ts should export typed constraint")
	assertContains(t, formDataContentFU, "FormDataValue", "form-data.ts should export FormDataValue type")
}

func TestControllerDirName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"OrdersController", "orders"},
		{"ProductsController", "products"},
		{"UserProfileController", "user-profile"},
		{"Controller", "default"},
		{"Health", "health"},
		{"Cart RecoveryController", "cart-recovery"},
		{"AdminDashboardController", "admin-dashboard"},
		{"FacebookAdsController", "facebook-ads"},
		{"CodReconciliationController", "cod-reconciliation"},
	}
	for _, tt := range tests {
		got := controllerDirName(tt.name)
		if got != tt.want {
			t.Errorf("controllerDirName(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestControllerPropertyName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"OrdersController", "orders"},
		{"UserProfileController", "userProfile"},
		{"Controller", "default"},
	}
	for _, tt := range tests {
		got := controllerPropertyName(tt.name)
		if got != tt.want {
			t.Errorf("controllerPropertyName(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestWriteFile_SkipUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.ts")

	// First write
	if err := writeFile(path, "export const x = 1;"); err != nil {
		t.Fatalf("first writeFile: %v", err)
	}

	// Get mtime after first write
	info1, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after first write: %v", err)
	}

	// Write same content — should be skipped
	if err := writeFile(path, "export const x = 1;"); err != nil {
		t.Fatalf("second writeFile: %v", err)
	}

	info2, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after second write: %v", err)
	}

	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Error("expected mtime to be unchanged when content is identical")
	}
}

func TestWriteFile_UpdateChanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.ts")

	if err := writeFile(path, "export const x = 1;"); err != nil {
		t.Fatalf("first writeFile: %v", err)
	}

	// Write different content — should update
	if err := writeFile(path, "export const x = 2;"); err != nil {
		t.Fatalf("second writeFile: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(content) != "export const x = 2;" {
		t.Errorf("expected updated content, got %q", string(content))
	}
}

func TestGenerate_EmptyFixture(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/empty.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Should still generate valid output
	typesContent := readFile(t, filepath.Join(outputDir, "types.ts"))
	assertContains(t, typesContent, "export {}", "empty fixture should produce export {}")
}

func TestGenerate_DeeplyNestedFixture(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/deeply-nested.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	typesContent := readFile(t, filepath.Join(outputDir, "types.ts"))
	assertContains(t, typesContent, "export interface Order", "should have Order interface")
	assertContains(t, typesContent, "export interface Shipping", "should have Shipping interface")
	assertContains(t, typesContent, "export interface Address", "should have Address interface")
	assertContains(t, typesContent, "export interface Coordinates", "should have Coordinates interface")

	// Array of arrays: string[][]
	assertContains(t, typesContent, "string[][]", "should have string[][] for tags")
}

func TestGenerate_ClientHandlesEmptyResponseBody(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/basic.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	clientContent := readFile(t, filepath.Join(outputDir, "client.ts"))

	// The generated client should read response body as text first before JSON.parse
	// to handle HTTP 200 with empty body gracefully (issue #89)
	assertContains(t, clientContent, "const text = await response.text();", "client.ts should read body as text before JSON parsing")
	assertContains(t, clientContent, "JSON.parse(text)", "client.ts should use JSON.parse(text) instead of response.json()")

	// Should NOT contain response.json() in the success path (only in error path)
	// Count occurrences: should only appear in the error handling block
	successBlock := clientContent[strings.Index(clientContent, "// Default: parse based on content-type header"):]
	assertNotContains(t, successBlock, "response.json()", "success path should not use response.json() directly")

	// Should check for empty text before parsing
	assertContains(t, clientContent, "if (!text)", "client.ts should check for empty body before JSON.parse")

	// JSON.parse should be wrapped in try-catch for malformed JSON
	assertContains(t, clientContent, "Invalid JSON response", "client.ts should catch malformed JSON and return error")
}

func TestGenerate_ClientCatchesNetworkErrors(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/basic.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	clientContent := readFile(t, filepath.Join(outputDir, "client.ts"))

	// Should have top-level try-catch for network errors, headers throws, etc.
	assertContains(t, clientContent, "} catch (err) {", "client.ts should have top-level try-catch")
	assertContains(t, clientContent, "err instanceof Error", "client.ts should extract Error.message")
	assertContains(t, clientContent, "status: 0", "client.ts should use status 0 for non-HTTP errors")
	assertContains(t, clientContent, "response: null", "client.ts should return null response for network errors")

	// SDKResult type should allow null response on error branch
	assertContains(t, clientContent, "response: Response | null", "SDKResult error branch should have nullable response")
}

func TestGenerate_EdgeCasesFixture(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/edge-cases.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Check controller output
	ctrlContent := readFile(t, filepath.Join(outputDir, "items/index.ts"))

	// Deprecated method should have @deprecated JSDoc
	assertContains(t, ctrlContent, "@deprecated", "deprecated method should have @deprecated JSDoc")

	// Required query param
	assertContains(t, ctrlContent, "status", "should have required status query param")

	// Optional request body
	assertContains(t, ctrlContent, "body?", "optional body should have ?")

	// SSE response should coexist with JSON responses
	assertContains(t, ctrlContent, "SSEConnection", "should have SSEConnection for SSE method")

	// HTML response should produce string type
	assertContains(t, ctrlContent, "SDKResult<string>", "text/html response should be string")

	// Image response should produce Blob type
	assertContains(t, ctrlContent, "SDKResult<Blob>", "image/png response should be Blob")

	// Void fallback for no 2xx responses
	assertContains(t, ctrlContent, "SDKResult<void>", "no 2xx responses should produce void")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func assertContains(t *testing.T, content, substr, msg string) {
	t.Helper()
	if !strings.Contains(content, substr) {
		t.Errorf("%s: expected %q in output:\n%s", msg, substr, truncate(content, 500))
	}
}

func assertNotContains(t *testing.T, content, substr, msg string) {
	t.Helper()
	if strings.Contains(content, substr) {
		t.Errorf("%s: unexpected %q in output:\n%s", msg, substr, truncate(content, 500))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func TestGenerate_AdvancedTypes(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/advanced-types.openapi.json"

	_, err := Generate(inputPath, outputDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	typesContent := readFile(t, filepath.Join(outputDir, "types.ts"))

	// --- Discriminated unions ---
	assertContains(t, typesContent, "Omit<Dog", "should have Omit<Dog for discriminated union")
	assertContains(t, typesContent, `"dog"`, "should have discriminator value 'dog'")
	assertContains(t, typesContent, "Omit<Cat", "should have Omit<Cat for discriminated union")
	assertContains(t, typesContent, `"cat"`, "should have discriminator value 'cat'")

	// --- readOnly ---
	assertContains(t, typesContent, "readonly barkVolume", "should have readonly modifier on readOnly property")

	// --- default ---
	assertContains(t, typesContent, "@default true", "should have @default JSDoc for Cat.indoor")

	// --- Nullable (3.1+ type array) ---
	assertContains(t, typesContent, "bio?: string | null", "should have nullable bio field (3.1+ type array)")

	// --- Nullable (3.0 flag) ---
	assertContains(t, typesContent, "legacyField?: string | null", "should have nullable legacyField (3.0 nullable flag)")

	// --- deprecated ---
	assertContains(t, typesContent, "@deprecated", "should have @deprecated JSDoc for oldApi")
	assertContains(t, typesContent, "Use newApi instead", "should preserve description alongside @deprecated")

	// --- Tuple (prefixItems) ---
	assertContains(t, typesContent, "[number, number]", "should have tuple type for Coordinate")

	// --- Enum (union type by default) ---
	assertContains(t, typesContent, `"pending"`, "should have enum value 'pending'")
	assertContains(t, typesContent, `"in-progress"`, "should have enum value 'in-progress'")
}

func TestGenerate_AdvancedTypes_TSEnums(t *testing.T) {
	outputDir := t.TempDir()
	inputPath := "../../testdata/sdkgen/advanced-types.openapi.json"

	_, err := Generate(inputPath, outputDir, &GenerateOptions{TSEnums: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	typesContent := readFile(t, filepath.Join(outputDir, "types.ts"))

	// --- TS enum ---
	assertContains(t, typesContent, "export enum StatusEnum", "should have TS enum declaration")
	assertContains(t, typesContent, `Pending = "pending"`, "should have Pending enum member")
	assertContains(t, typesContent, `InProgress = "in-progress"`, "should have InProgress enum member")
	assertContains(t, typesContent, `Completed = "completed"`, "should have Completed enum member")
	assertContains(t, typesContent, `Archived = "archived"`, "should have Archived enum member")
	// Non-enum types should still be interfaces
	assertContains(t, typesContent, "export interface Dog", "Dog should still be an interface")
}
