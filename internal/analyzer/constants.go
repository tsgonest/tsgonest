package analyzer

// HttpMethod represents an HTTP method recognized by NestJS route decorators.
type HttpMethod string

const (
	MethodGet     HttpMethod = "GET"
	MethodPost    HttpMethod = "POST"
	MethodPut     HttpMethod = "PUT"
	MethodDelete  HttpMethod = "DELETE"
	MethodPatch   HttpMethod = "PATCH"
	MethodHead    HttpMethod = "HEAD"
	MethodOptions HttpMethod = "OPTIONS"
)

// AllHttpMethods is the set of all HTTP methods used by @All() decorator expansion.
var AllHttpMethods = []string{
	string(MethodGet), string(MethodPost), string(MethodPut),
	string(MethodDelete), string(MethodPatch), string(MethodHead),
	string(MethodOptions),
}

// ParamCategory identifies the source of a route parameter.
type ParamCategory string

const (
	CategoryBody    ParamCategory = "body"
	CategoryQuery   ParamCategory = "query"
	CategoryParam   ParamCategory = "param"
	CategoryHeaders ParamCategory = "headers"
)

// WarningKind identifies the type of warning emitted during controller analysis.
type WarningKind string

const (
	WarnUnsupportedRuntimeController     WarningKind = "unsupported-runtime-controller"
	WarnUnsupportedDynamicControllerPath WarningKind = "unsupported-dynamic-controller-path"
	WarnUnsupportedDynamicRoutePath      WarningKind = "unsupported-dynamic-route-path"
	WarnUsesRawResponse                  WarningKind = "uses-raw-response"
	WarnParamNonScalar                   WarningKind = "param-non-scalar"
	WarnParamAny                         WarningKind = "param-any"
	WarnParamOptional                    WarningKind = "param-optional"
	WarnParamUnion                       WarningKind = "param-union"
	WarnParamNoName                      WarningKind = "param-no-name"
	WarnCustomDecoratorNoName            WarningKind = "custom-decorator-no-name"
	WarnParamComplexType                 WarningKind = "param-complex-type"
	WarnQueryComplexType                 WarningKind = "query-complex-type"
	WarnQueryNullable                    WarningKind = "query-nullable"
	WarnHeaderNull                       WarningKind = "header-null"
	WarnHeaderComplexType                WarningKind = "header-complex-type"
	WarnSlowReturnTypeInference          WarningKind = "slow-return-type-inference"
	WarnUnresolvableReturnType           WarningKind = "unresolvable-return-type"
)
