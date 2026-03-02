package constraints

import (
	"reflect"
	"testing"

	"github.com/tsgonest/tsgonest/internal/metadata"
)

// TestSetBranded_AllFieldsCovered verifies that SetBranded handles every branded
// type constraint key. Each key in brandedKeys should set at least one field
// on metadata.Constraints when given appropriate input.
func TestSetBranded_AllFieldsCovered(t *testing.T) {
	type testCase struct {
		key      string
		meta     *metadata.Metadata
		field    string // Constraints field name to check
		wantBool bool   // for boolean fields, expect this
	}

	strLit := func(s string) *metadata.Metadata {
		return &metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: s}
	}
	intLit := func(n int) *metadata.Metadata {
		return &metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: float64(n)}
	}
	floatLit := func(f float64) *metadata.Metadata {
		return &metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: f}
	}
	boolLit := func(b bool) *metadata.Metadata {
		return &metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: b}
	}

	tests := []testCase{
		// String constraints
		{key: "format", meta: strLit("email"), field: "Format"},
		{key: "minLength", meta: intLit(1), field: "MinLength"},
		{key: "maxLength", meta: intLit(100), field: "MaxLength"},
		{key: "pattern", meta: strLit("^[a-z]+$"), field: "Pattern"},
		{key: "startsWith", meta: strLit("https://"), field: "StartsWith"},
		{key: "endsWith", meta: strLit(".com"), field: "EndsWith"},
		{key: "includes", meta: strLit("@"), field: "Includes"},

		// Numeric constraints
		{key: "minimum", meta: floatLit(0), field: "Minimum"},
		{key: "maximum", meta: floatLit(100), field: "Maximum"},
		{key: "exclusiveMinimum", meta: floatLit(0), field: "ExclusiveMinimum"},
		{key: "exclusiveMaximum", meta: floatLit(100), field: "ExclusiveMaximum"},
		{key: "multipleOf", meta: floatLit(5), field: "MultipleOf"},
		{key: "type", meta: strLit("int32"), field: "NumericType"},

		// Array constraints
		{key: "minItems", meta: intLit(1), field: "MinItems"},
		{key: "maxItems", meta: intLit(10), field: "MaxItems"},
		{key: "uniqueItems", meta: boolLit(true), field: "UniqueItems"},

		// String case validation
		{key: "uppercase", meta: boolLit(true), field: "Uppercase"},
		{key: "lowercase", meta: boolLit(true), field: "Lowercase"},

		// Meta
		{key: "error", meta: strLit("custom error"), field: "ErrorMessage"},
		{key: "default", meta: strLit("hello"), field: "Default"},
		{key: "coerce", meta: boolLit(true), field: "Coerce"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			c := &metadata.Constraints{}
			ok := SetBranded(c, tt.key, tt.meta)
			if !ok {
				t.Fatalf("SetBranded(%q) returned false", tt.key)
			}

			// Use reflection to verify the field was set
			v := reflect.ValueOf(c).Elem()
			f := v.FieldByName(tt.field)
			if !f.IsValid() {
				t.Fatalf("no field %q on Constraints", tt.field)
			}
			if f.IsNil() {
				t.Fatalf("field %q is nil after SetBranded(%q)", tt.field, tt.key)
			}
		})
	}
}

// TestSetBranded_Transforms verifies transform keys append to Transforms slice.
func TestSetBranded_Transforms(t *testing.T) {
	tests := []struct {
		key       string
		wantValue string
	}{
		{"transform_trim", string(TransformTrim)},
		{"transform_toLowerCase", string(TransformToLowerCase)},
		{"transform_toUpperCase", string(TransformToUpperCase)},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			c := &metadata.Constraints{}
			boolTrue := &metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: true}
			ok := SetBranded(c, tt.key, boolTrue)
			if !ok {
				t.Fatalf("SetBranded(%q) returned false", tt.key)
			}
			if len(c.Transforms) != 1 || c.Transforms[0] != tt.wantValue {
				t.Fatalf("expected Transforms=[%q], got %v", tt.wantValue, c.Transforms)
			}
		})
	}
}

// TestSetBranded_UnknownKey verifies unknown keys return false.
func TestSetBranded_UnknownKey(t *testing.T) {
	c := &metadata.Constraints{}
	ok := SetBranded(c, "nonexistent", &metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "x"})
	if ok {
		t.Fatal("expected false for unknown key")
	}
}

// TestMerge_AllFields verifies Merge copies all non-nil fields from src to dst.
func TestMerge_AllFields(t *testing.T) {
	f := 42.0
	n := 10
	s := "test"
	b := true

	src := &metadata.Constraints{
		Minimum:          &f,
		Maximum:          &f,
		ExclusiveMinimum: &f,
		ExclusiveMaximum: &f,
		MultipleOf:       &f,
		NumericType:      &s,
		MinLength:        &n,
		MaxLength:        &n,
		Pattern:          &s,
		Format:           &s,
		StartsWith:       &s,
		EndsWith:         &s,
		Includes:         &s,
		Uppercase:        &b,
		Lowercase:        &b,
		ContentMediaType: &s,
		Transforms:       []string{"trim"},
		MinItems:         &n,
		MaxItems:         &n,
		UniqueItems:      &b,
		Default:          &s,
		Coerce:           &b,
		ValidateFn:       &s,
		ValidateModule:   &s,
		ErrorMessage:     &s,
		Errors:           map[string]string{"format": "bad format"},
	}

	dst := &metadata.Constraints{}
	Merge(dst, src)

	// Verify all pointer fields were copied
	dstV := reflect.ValueOf(dst).Elem()
	srcV := reflect.ValueOf(src).Elem()
	dstT := dstV.Type()

	for i := 0; i < dstT.NumField(); i++ {
		fieldName := dstT.Field(i).Name
		dstField := dstV.Field(i)
		srcField := srcV.Field(i)

		switch dstField.Kind() {
		case reflect.Ptr:
			if srcField.IsNil() {
				continue
			}
			if dstField.IsNil() {
				t.Errorf("Merge did not copy field %s", fieldName)
			}
		case reflect.Slice:
			if srcField.Len() > 0 && dstField.Len() == 0 {
				t.Errorf("Merge did not copy slice field %s", fieldName)
			}
		case reflect.Map:
			if srcField.Len() > 0 && dstField.Len() == 0 {
				t.Errorf("Merge did not copy map field %s", fieldName)
			}
		}
	}
}

// TestMerge_ErrorsMapMerge verifies Errors maps are merged, not replaced.
func TestMerge_ErrorsMapMerge(t *testing.T) {
	dst := &metadata.Constraints{
		Errors: map[string]string{"minimum": "too small"},
	}
	src := &metadata.Constraints{
		Errors: map[string]string{"maximum": "too big"},
	}

	Merge(dst, src)

	if len(dst.Errors) != 2 {
		t.Fatalf("expected 2 error entries, got %d", len(dst.Errors))
	}
	if dst.Errors["minimum"] != "too small" {
		t.Error("dst.Errors[minimum] was overwritten")
	}
	if dst.Errors["maximum"] != "too big" {
		t.Error("dst.Errors[maximum] was not merged")
	}
}

// TestMerge_ErrorsMapOverride verifies src Errors override dst Errors for same key.
func TestMerge_ErrorsMapOverride(t *testing.T) {
	dst := &metadata.Constraints{
		Errors: map[string]string{"format": "old"},
	}
	src := &metadata.Constraints{
		Errors: map[string]string{"format": "new"},
	}

	Merge(dst, src)

	if dst.Errors["format"] != "new" {
		t.Errorf("expected 'new', got %q", dst.Errors["format"])
	}
}

// TestLiteralHelpers verifies all literal extraction helpers.
func TestLiteralHelpers(t *testing.T) {
	t.Run("LiteralString", func(t *testing.T) {
		s, ok := LiteralString(&metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: "hello"})
		if !ok || s != "hello" {
			t.Fatalf("expected (hello, true), got (%s, %v)", s, ok)
		}
		_, ok = LiteralString(&metadata.Metadata{Kind: metadata.KindAtomic})
		if ok {
			t.Fatal("expected false for non-literal")
		}
	})

	t.Run("LiteralFloat", func(t *testing.T) {
		f, ok := LiteralFloat(&metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: 3.14})
		if !ok || f != 3.14 {
			t.Fatalf("expected (3.14, true), got (%f, %v)", f, ok)
		}
		f, ok = LiteralFloat(&metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: int(42)})
		if !ok || f != 42 {
			t.Fatal("int conversion failed")
		}
		f, ok = LiteralFloat(&metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: int64(99)})
		if !ok || f != 99 {
			t.Fatal("int64 conversion failed")
		}
	})

	t.Run("LiteralInt", func(t *testing.T) {
		n, ok := LiteralInt(&metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: float64(5)})
		if !ok || n != 5 {
			t.Fatalf("expected (5, true), got (%d, %v)", n, ok)
		}
	})

	t.Run("LiteralBool", func(t *testing.T) {
		b, ok := LiteralBool(&metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: true})
		if !ok || !b {
			t.Fatal("expected (true, true)")
		}
	})
}

// TestValidNumericTypes verifies the set matches the NumericType constants.
func TestValidNumericTypes(t *testing.T) {
	expected := []NumericType{
		NumericInt32, NumericUint32, NumericInt64,
		NumericUint64, NumericFloat, NumericDouble,
	}
	for _, nt := range expected {
		if !ValidNumericTypes[string(nt)] {
			t.Errorf("ValidNumericTypes missing %q", nt)
		}
	}
	if len(ValidNumericTypes) != len(expected) {
		t.Errorf("ValidNumericTypes has %d entries, expected %d", len(ValidNumericTypes), len(expected))
	}
}

// TestSetBranded_DefaultNumericAndBool verifies default field handles
// numeric and boolean literal values (not just strings).
func TestSetBranded_DefaultNumericAndBool(t *testing.T) {
	c := &metadata.Constraints{}
	ok := SetBranded(c, "default", &metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: float64(42)})
	if !ok || c.Default == nil || *c.Default != "42" {
		t.Fatalf("numeric default: ok=%v, Default=%v", ok, c.Default)
	}

	c = &metadata.Constraints{}
	ok = SetBranded(c, "default", &metadata.Metadata{Kind: metadata.KindLiteral, LiteralValue: true})
	if !ok || c.Default == nil || *c.Default != "true" {
		t.Fatalf("bool default: ok=%v, Default=%v", ok, c.Default)
	}
}
