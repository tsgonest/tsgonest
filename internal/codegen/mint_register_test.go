package codegen

import (
	"strings"
	"testing"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

func TestGenerateMintRegister_SingleGetRoute(t *testing.T) {
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "HelloController",
		ControllerImportPath: "./hello.controller",
		Routes: []MintRouteInfo{
			{Method: "GET", Path: "/hello", MethodName: "hello", ReturnAtomic: "string"},
		},
	})

	wants := []string{
		`import { HelloController } from "./hello.controller";`,
		`export function registerHelloController(app) {`,
		`app.router.add("GET", "/hello", async (event) => {`,
		`const ctrl = event.resolve(HelloController);`,
		`const result = await ctrl.hello();`,
		`if (result instanceof Response) return result;`,
	}
	for _, w := range wants {
		assertContains(t, out, w)
	}
}

func TestGenerateMintRegister_MultipleRoutes(t *testing.T) {
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "UsersController",
		ControllerImportPath: "./users.controller",
		Routes: []MintRouteInfo{
			{Method: "GET", Path: "/users", MethodName: "list"},
			{Method: "POST", Path: "/users", MethodName: "create"},
			{Method: "DELETE", Path: "/users/:id", MethodName: "remove"},
		},
	})

	assertContains(t, out, `export function registerUsersController(app) {`)
	assertContains(t, out, `app.router.add("GET", "/users", async (event) => {`)
	assertContains(t, out, `app.router.add("POST", "/users", async (event) => {`)
	assertContains(t, out, `app.router.add("DELETE", "/users/:id", async (event) => {`)
}

func TestGenerateMintRegister_ResponsePassthrough(t *testing.T) {
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "PingController",
		ControllerImportPath: "./ping.controller",
		Routes: []MintRouteInfo{
			{Method: "GET", Path: "/ping", MethodName: "ping"},
		},
	})

	assertContains(t, out, "if (result instanceof Response) return result;")
}

func TestGenerateMintRegister_KeepsControllerSuffix(t *testing.T) {
	// Phase 1: do NOT strip "Controller" suffix.
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "HelloController",
		ControllerImportPath: "./hello.controller",
		Routes:               []MintRouteInfo{{Method: "GET", Path: "/", MethodName: "hello"}},
	})
	assertContains(t, out, "registerHelloController")
}

func TestGenerateMintRegisterTypes(t *testing.T) {
	out := GenerateMintRegisterTypes("HelloController")
	assertContains(t, out, "export declare function registerHelloController(app: any): void;")
}

func TestGenerateMintRegister_BodyParam_ImportsAndAsserts(t *testing.T) {
	// @Body() body: CreateUserDto → parse JSON, assertCreateUserDto, pass to handler.
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "UsersController",
		ControllerImportPath: "./users.controller",
		Routes: []MintRouteInfo{
			{
				Method:     "POST",
				Path:       "/users",
				MethodName: "create",
				Params: []MintParamInfo{
					{
						Kind:            MintParamBody,
						LocalName:       "body",
						TypeName:        "CreateUserDto",
						CompanionImport: "./users.dto.CreateUserDto.tsgonest.js",
					},
				},
				ReturnTypeName:        "UserResponse",
				ReturnCompanionImport: "./users.dto.UserResponse.tsgonest.js",
			},
		},
	})

	assertContains(t, out, `import { assertCreateUserDto } from "./users.dto.CreateUserDto.tsgonest.js";`)
	assertContains(t, out, `import { stringifyUserResponse } from "./users.dto.UserResponse.tsgonest.js";`)
	assertContains(t, out, `const body = assertCreateUserDto(await event.body.json());`)
	assertContains(t, out, `await ctrl.create(body)`)
	assertContains(t, out, `stringifyUserResponse(`)
	assertContains(t, out, `"content-type"`)
}

func TestGenerateMintRegister_QueryDtoParam(t *testing.T) {
	// @Query() q: ListQuery → parse searchParams, assertListQuery.
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "UsersController",
		ControllerImportPath: "./users.controller",
		Routes: []MintRouteInfo{
			{
				Method:     "GET",
				Path:       "/users",
				MethodName: "list",
				Params: []MintParamInfo{
					{
						Kind:            MintParamQuery,
						LocalName:       "q",
						TypeName:        "ListQuery",
						CompanionImport: "./users.dto.ListQuery.tsgonest.js",
					},
				},
			},
		},
	})

	assertContains(t, out, `import { assertListQuery } from "./users.dto.ListQuery.tsgonest.js";`)
	// Expect a parse of searchParams into a plain object before assert.
	assertContains(t, out, `new URL(event.request.url)`)
	assertContains(t, out, `assertListQuery(`)
	assertContains(t, out, `await ctrl.list(q)`)
}

func TestGenerateMintRegister_NamedQueryScalarString_WithConstraints(t *testing.T) {
	// @Query('name') name: string & tags.MinLength<1> → searchParams.get + minLength check.
	min := 1
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "ItemsController",
		ControllerImportPath: "./items.controller",
		Routes: []MintRouteInfo{
			{
				Method:     "GET",
				Path:       "/items",
				MethodName: "search",
				Params: []MintParamInfo{
					{
						Kind:      MintParamQuery,
						Name:      "name",
						LocalName: "name",
						Atomic:    "string",
						Constraints: &metadata.Constraints{
							MinLength: &min,
						},
					},
				},
			},
		},
	})

	assertContains(t, out, `searchParams.get("name")`)
	// minLength constraint emitted.
	assertContains(t, out, `name.length < 1`)
	assertContains(t, out, `await ctrl.search(name)`)
}

func TestGenerateMintRegister_NamedParamScalarString(t *testing.T) {
	// @Param('id') id: string → extract from event.params, no coercion.
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "UsersController",
		ControllerImportPath: "./users.controller",
		Routes: []MintRouteInfo{
			{
				Method:     "GET",
				Path:       "/users/:id",
				MethodName: "one",
				Params: []MintParamInfo{
					{
						Kind:      MintParamPathParam,
						Name:      "id",
						LocalName: "id",
						Atomic:    "string",
					},
				},
			},
		},
	})

	assertContains(t, out, `event.params["id"]`)
	assertContains(t, out, `await ctrl.one(id)`)
}

func TestGenerateMintRegister_NamedQueryScalarNumber_Coerces(t *testing.T) {
	// @Query('limit') limit: number & tags.Maximum<100>
	max := 100.0
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "ItemsController",
		ControllerImportPath: "./items.controller",
		Routes: []MintRouteInfo{
			{
				Method:     "GET",
				Path:       "/items",
				MethodName: "list",
				Params: []MintParamInfo{
					{
						Kind:      MintParamQuery,
						Name:      "limit",
						LocalName: "limit",
						Atomic:    "number",
						Constraints: &metadata.Constraints{
							Maximum: &max,
						},
					},
				},
			},
		},
	})

	assertContains(t, out, `searchParams.get("limit")`)
	// Coerce string → number.
	assertContains(t, out, `+limit`)
	// Constraint emitted after coercion.
	assertContains(t, out, `limit > 100`)
	assertContains(t, out, `await ctrl.list(limit)`)
}

func TestGenerateMintRegister_NamedHeader(t *testing.T) {
	// @Headers('x-foo') h: string → request.headers.get(name).
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "PingController",
		ControllerImportPath: "./ping.controller",
		Routes: []MintRouteInfo{
			{
				Method:     "GET",
				Path:       "/ping",
				MethodName: "ping",
				Params: []MintParamInfo{
					{
						Kind:      MintParamHeader,
						Name:      "x-foo",
						LocalName: "h",
						Atomic:    "string",
					},
				},
			},
		},
	})

	assertContains(t, out, `event.request.headers.get("x-foo")`)
	assertContains(t, out, `await ctrl.ping(h)`)
}

func TestGenerateMintRegister_ArrayReturnUsesSerialize(t *testing.T) {
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "UsersController",
		ControllerImportPath: "./users.controller",
		Routes: []MintRouteInfo{
			{
				Method:                "GET",
				Path:                  "/users",
				MethodName:            "list",
				ReturnTypeName:        "UserResponse",
				ReturnIsArray:         true,
				ReturnCompanionImport: "./users.dto.UserResponse.tsgonest.js",
			},
		},
	})

	// Array returns: validate elements with assert + use serialize+__sa per array; we emit serializeUserResponse + array wrap.
	assertContains(t, out, `serializeUserResponse`)
}

func TestGenerateMintRegister_PrimitiveStringReturnJsonStringifies(t *testing.T) {
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "HelloController",
		ControllerImportPath: "./hello.controller",
		Routes: []MintRouteInfo{
			{
				Method:       "GET",
				Path:         "/hello",
				MethodName:   "hello",
				ReturnAtomic: "string",
			},
		},
	})

	// No DTO: must JSON.stringify the primitive and set application/json.
	assertContains(t, out, "JSON.stringify(result)")
	assertContains(t, out, `"content-type"`)
	assertContains(t, out, `application/json`)
}

func TestGenerateMintRegister_VoidReturn204(t *testing.T) {
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "UsersController",
		ControllerImportPath: "./users.controller",
		Routes: []MintRouteInfo{
			{
				Method:     "DELETE",
				Path:       "/users/:id",
				MethodName: "remove",
				ReturnVoid: true,
			},
		},
	})

	// Void return: 204 No Content, no body.
	if !strings.Contains(out, "204") {
		t.Errorf("expected 204 status for void return, got:\n%s", out)
	}
}

func TestGenerateMintRegister_BufferedFileUpload(t *testing.T) {
	// @Body() data: { title: string; image: File & MaxSize<5000> & MimeTypes<'image/png'> }
	// → multipart path: read event.body.formData(), validate per field.
	min := 1
	maxSize := uint64(5000)
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "UploadController",
		ControllerImportPath: "./upload.controller",
		Routes: []MintRouteInfo{
			{
				Method:     "POST",
				Path:       "/avatar",
				MethodName: "upload",
				Params: []MintParamInfo{
					{
						Kind:      MintParamBody,
						LocalName: "data",
						Multipart: &MintMultipartBody{
							Streaming: false,
							Fields: []MintMultipartField{
								{
									Name:     "title",
									Kind:     MintFieldScalar,
									Atomic:   "string",
									Required: true,
									Constraints: &metadata.Constraints{
										MinLength: &min,
									},
								},
								{
									Name:     "image",
									Kind:     MintFieldFile,
									Required: true,
									Constraints: &metadata.Constraints{
										MaxSize:   &maxSize,
										MimeTypes: []string{"image/png"},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	wants := []string{
		// Multipart path.
		`event.body.formData()`,
		// Scalar field extraction.
		`__form.get("title")`,
		// MinLength constraint.
		`__raw.length < 1`,
		// File extraction.
		`__form.get("image")`,
		// MaxSize check.
		`.size > 5000`,
		// Mime helper import and call.
		`matchMimeType`,
		// JSON parse path NOT used.
	}
	for _, w := range wants {
		assertContains(t, out, w)
	}
	if strings.Contains(out, "event.body.json()") {
		t.Errorf("expected NO JSON body parse when multipart; got:\n%s", out)
	}
}

func TestGenerateMintRegister_BufferedFileArray(t *testing.T) {
	// photos: Array<File & MaxSize<10000> & MimeTypes<'image/*'>>
	maxSize := uint64(10000)
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "GalleryController",
		ControllerImportPath: "./gallery.controller",
		Routes: []MintRouteInfo{
			{
				Method:     "POST",
				Path:       "/gallery",
				MethodName: "upload",
				Params: []MintParamInfo{
					{
						Kind:      MintParamBody,
						LocalName: "data",
						Multipart: &MintMultipartBody{
							Fields: []MintMultipartField{
								{
									Name:     "photos",
									Kind:     MintFieldFileArray,
									Required: true,
									Constraints: &metadata.Constraints{
										MaxSize:   &maxSize,
										MimeTypes: []string{"image/*"},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	// getAll for arrays + per-element validation.
	assertContains(t, out, `__form.getAll("photos")`)
	assertContains(t, out, `for (let __i = 0; __i < __files.length`)
	assertContains(t, out, `.size > 10000`)
	assertContains(t, out, `matchMimeType`)
	assertContains(t, out, `"image/*"`)
}

func TestGenerateMintRegister_StreamingFileUpload(t *testing.T) {
	maxSize := uint64(5_000_000_000)
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "LargeUploadController",
		ControllerImportPath: "./large.controller",
		Routes: []MintRouteInfo{
			{
				Method:     "POST",
				Path:       "/uploads/large",
				MethodName: "upload",
				Params: []MintParamInfo{
					{
						Kind:      MintParamBody,
						LocalName: "data",
						Multipart: &MintMultipartBody{
							Streaming: true,
							Fields: []MintMultipartField{
								{
									Name:     "filename",
									Kind:     MintFieldScalar,
									Atomic:   "string",
									Required: true,
								},
								{
									Name:     "file",
									Kind:     MintFieldFileStream,
									Required: true,
									Constraints: &metadata.Constraints{
										MaxSize: &maxSize,
									},
								},
							},
						},
					},
				},
			},
		},
	})

	wants := []string{
		`parseMultipartStream`,
		`event.body.stream()`,
		`__mint_multipart_boundary`,
		// FileStream binding.
		`stream: __part.stream`,
		// setLimit invocation on the parser.
		`__iter.setLimit(5000000000`,
		`MultipartByteLimitError`,
	}
	for _, w := range wants {
		assertContains(t, out, w)
	}
}

func TestGenerateMintRegister_NoneResponseSerializer_UsesPlainJsonStringify(t *testing.T) {
	out := GenerateMintRegister(MintRegisterInput{
		ControllerName:       "UsersController",
		ControllerImportPath: "./users.controller",
		ResponseSerializer:   "none",
		Routes: []MintRouteInfo{
			{
				Method:                "GET",
				Path:                  "/users",
				MethodName:            "list",
				ReturnTypeName:        "UserResponse",
				ReturnCompanionImport: "./users.dto.UserResponse.tsgonest.js",
			},
		},
	})

	// When responseSerializer="none", emit JSON.stringify(result) — no companion import for return.
	assertContains(t, out, "JSON.stringify(result)")
	// Do not import the companion's stringify when serializer is "none".
	if strings.Contains(out, "stringifyUserResponse") {
		t.Errorf("expected no stringifyUserResponse when serializer=none, got:\n%s", out)
	}
}
