package analyzer_test

import (
	"testing"

	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

func TestAutoEnableCoercion_SetsCoerceOnArrayProperty(t *testing.T) {
	m := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "ids",
				Type: metadata.Metadata{
					Kind:        metadata.KindArray,
					ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
				},
			},
		},
	}

	analyzer.AutoEnableCoercion(m)

	prop := m.Properties[0]
	if prop.Constraints == nil || prop.Constraints.Coerce == nil || !*prop.Constraints.Coerce {
		t.Error("AutoEnableCoercion should set Coerce=true on array properties with coercible element types")
	}
}

func TestAutoEnableCoercion_SetsCoerceOnArrayOfBooleans(t *testing.T) {
	m := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "flags",
				Type: metadata.Metadata{
					Kind:        metadata.KindArray,
					ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "boolean"},
				},
			},
		},
	}

	analyzer.AutoEnableCoercion(m)

	prop := m.Properties[0]
	if prop.Constraints == nil || prop.Constraints.Coerce == nil || !*prop.Constraints.Coerce {
		t.Error("AutoEnableCoercion should set Coerce=true on array properties with boolean elements")
	}
}

func TestAutoEnableCoercion_NoCoerceOnArrayOfStrings(t *testing.T) {
	m := &metadata.Metadata{
		Kind: metadata.KindObject,
		Properties: []metadata.Property{
			{
				Name: "tags",
				Type: metadata.Metadata{
					Kind:        metadata.KindArray,
					ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "string"},
				},
			},
		},
	}

	analyzer.AutoEnableCoercion(m)

	prop := m.Properties[0]
	// String arrays still need wrapping (single value → array), so Coerce should be set
	if prop.Constraints == nil || prop.Constraints.Coerce == nil || !*prop.Constraints.Coerce {
		t.Error("AutoEnableCoercion should set Coerce=true on array properties (for single-value wrapping)")
	}
}

func TestAutoEnableCoercion_ArrayElementCoercion(t *testing.T) {
	m := &metadata.Metadata{
		Kind: metadata.KindArray,
		ElementType: &metadata.Metadata{Kind: metadata.KindAtomic, Atomic: "number"},
	}

	analyzer.AutoEnableCoercion(m)

	// Element type should have coercion enabled
	if m.ElementType.Constraints == nil || m.ElementType.Constraints.Coerce == nil || !*m.ElementType.Constraints.Coerce {
		t.Error("AutoEnableCoercion should set Coerce=true on array element type when it's number")
	}
}
