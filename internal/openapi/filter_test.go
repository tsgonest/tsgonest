package openapi

import (
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
)

func makeControllers() []analyzer.ControllerInfo {
	return []analyzer.ControllerInfo{
		{
			Name:       "PublicUsersController",
			SourceFile: "/project/src/public/users.controller.ts",
			Routes: []analyzer.Route{
				{MethodName: "list", Tags: []string{"public", "users"}},
				{MethodName: "get", Tags: []string{"public", "users"}},
			},
		},
		{
			Name:       "InternalUsersController",
			SourceFile: "/project/src/internal/users.controller.ts",
			Routes: []analyzer.Route{
				{MethodName: "listAll", Tags: []string{"internal", "users"}},
				{MethodName: "delete", Tags: []string{"internal", "users", "admin"}},
			},
		},
		{
			Name:       "MixedController",
			SourceFile: "/project/src/mixed/mixed.controller.ts",
			Routes: []analyzer.Route{
				{MethodName: "publicEndpoint", Tags: []string{"public"}},
				{MethodName: "internalEndpoint", Tags: []string{"internal"}},
				{MethodName: "deprecatedEndpoint", Tags: []string{"public", "deprecated"}},
			},
		},
	}
}

func TestFilterControllers_NoFilters(t *testing.T) {
	controllers := makeControllers()
	result := FilterControllers(controllers, FilterOptions{})

	if len(result) != 3 {
		t.Fatalf("expected 3 controllers, got %d", len(result))
	}
}

func TestFilterControllers_GlobInclude(t *testing.T) {
	controllers := makeControllers()
	result := FilterControllers(controllers, FilterOptions{
		ControllerInclude: []string{"src/public/**/*.controller.ts"},
	})

	if len(result) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(result))
	}
	if result[0].Name != "PublicUsersController" {
		t.Fatalf("expected PublicUsersController, got %s", result[0].Name)
	}
}

func TestFilterControllers_GlobExclude(t *testing.T) {
	controllers := makeControllers()
	result := FilterControllers(controllers, FilterOptions{
		ControllerInclude: []string{"src/**/*.controller.ts"},
		ControllerExclude: []string{"src/internal/**/*.controller.ts"},
	})

	if len(result) != 2 {
		t.Fatalf("expected 2 controllers, got %d", len(result))
	}
	for _, c := range result {
		if c.Name == "InternalUsersController" {
			t.Fatal("InternalUsersController should have been excluded")
		}
	}
}

func TestFilterControllers_IncludeTags(t *testing.T) {
	controllers := makeControllers()
	result := FilterControllers(controllers, FilterOptions{
		IncludeTags: []string{"public"},
	})

	if len(result) != 2 {
		t.Fatalf("expected 2 controllers (PublicUsers + Mixed), got %d", len(result))
	}

	// MixedController should only have public routes
	for _, c := range result {
		if c.Name == "MixedController" {
			if len(c.Routes) != 2 {
				t.Fatalf("MixedController should have 2 public routes, got %d", len(c.Routes))
			}
		}
	}
}

func TestFilterControllers_ExcludeTags(t *testing.T) {
	controllers := makeControllers()
	result := FilterControllers(controllers, FilterOptions{
		ExcludeTags: []string{"deprecated"},
	})

	if len(result) != 3 {
		t.Fatalf("expected 3 controllers, got %d", len(result))
	}

	// MixedController should have 2 routes (deprecatedEndpoint removed)
	for _, c := range result {
		if c.Name == "MixedController" {
			if len(c.Routes) != 2 {
				t.Fatalf("MixedController should have 2 routes after excluding deprecated, got %d", len(c.Routes))
			}
			for _, r := range c.Routes {
				if r.MethodName == "deprecatedEndpoint" {
					t.Fatal("deprecatedEndpoint should have been excluded")
				}
			}
		}
	}
}

func TestFilterControllers_IncludeAndExcludeTags(t *testing.T) {
	controllers := makeControllers()
	result := FilterControllers(controllers, FilterOptions{
		IncludeTags: []string{"public"},
		ExcludeTags: []string{"deprecated"},
	})

	// MixedController should only have publicEndpoint (not deprecated)
	for _, c := range result {
		if c.Name == "MixedController" {
			if len(c.Routes) != 1 {
				t.Fatalf("MixedController should have 1 route, got %d", len(c.Routes))
			}
			if c.Routes[0].MethodName != "publicEndpoint" {
				t.Fatalf("expected publicEndpoint, got %s", c.Routes[0].MethodName)
			}
		}
	}
}

func TestFilterControllers_GlobAndTags(t *testing.T) {
	controllers := makeControllers()
	result := FilterControllers(controllers, FilterOptions{
		ControllerInclude: []string{"src/mixed/**/*.controller.ts"},
		IncludeTags:       []string{"public"},
	})

	if len(result) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(result))
	}
	if result[0].Name != "MixedController" {
		t.Fatalf("expected MixedController, got %s", result[0].Name)
	}
	// Should have 2 public routes (publicEndpoint + deprecatedEndpoint which is also tagged public)
	if len(result[0].Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(result[0].Routes))
	}
}

func TestFilterControllers_TagFilterRemovesEmptyControllers(t *testing.T) {
	controllers := makeControllers()
	result := FilterControllers(controllers, FilterOptions{
		IncludeTags: []string{"admin"},
	})

	// Only InternalUsersController has an "admin" tag (on the delete route)
	if len(result) != 1 {
		t.Fatalf("expected 1 controller with admin tag, got %d", len(result))
	}
	if result[0].Name != "InternalUsersController" {
		t.Fatalf("expected InternalUsersController, got %s", result[0].Name)
	}
	if len(result[0].Routes) != 1 {
		t.Fatalf("expected 1 admin route, got %d", len(result[0].Routes))
	}
}

func TestFilterControllers_DoesNotMutateOriginal(t *testing.T) {
	controllers := makeControllers()
	originalRouteCount := len(controllers[2].Routes) // MixedController has 3 routes

	_ = FilterControllers(controllers, FilterOptions{
		IncludeTags: []string{"public"},
	})

	// Original should be unchanged
	if len(controllers[2].Routes) != originalRouteCount {
		t.Fatalf("original controller was mutated: expected %d routes, got %d",
			originalRouteCount, len(controllers[2].Routes))
	}
}
