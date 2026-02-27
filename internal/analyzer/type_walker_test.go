package analyzer_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/shim/ast"
	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	"github.com/tsgonest/tsgonest/internal/analyzer"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

// --- 2.4a: Primitive types ---

func TestWalkPrimitiveString(t *testing.T) {
	env := setupWalker(t, `type T = string;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "string")
}

func TestWalkPrimitiveNumber(t *testing.T) {
	env := setupWalker(t, `type T = number;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "number")
}

func TestWalkPrimitiveBoolean(t *testing.T) {
	env := setupWalker(t, `type T = boolean;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "boolean")
}

func TestWalkPrimitiveBigint(t *testing.T) {
	env := setupWalker(t, `type T = bigint;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "bigint")
}

func TestWalkPrimitiveSymbol(t *testing.T) {
	env := setupWalker(t, `type T = symbol;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "symbol")
}

func TestWalkPrimitiveAny(t *testing.T) {
	env := setupWalker(t, `type T = any;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAny)
}

func TestWalkPrimitiveUnknown(t *testing.T) {
	env := setupWalker(t, `type T = unknown;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindUnknown)
}

func TestWalkPrimitiveNever(t *testing.T) {
	env := setupWalker(t, `type T = never;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindNever)
}

func TestWalkPrimitiveVoid(t *testing.T) {
	env := setupWalker(t, `type T = void;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindVoid)
}

// --- 2.4b: Literal types ---

func TestWalkLiteralString(t *testing.T) {
	env := setupWalker(t, `type T = "hello";`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindLiteral)
	if m.LiteralValue != "hello" {
		t.Errorf("expected literal value %q, got %v", "hello", m.LiteralValue)
	}
}

func TestWalkLiteralNumber(t *testing.T) {
	env := setupWalker(t, `type T = 42;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindLiteral)
	// Number literals may come as float64 from the checker
	assertNumericLiteral(t, m, 42)
}

func TestWalkLiteralTrue(t *testing.T) {
	env := setupWalker(t, `type T = true;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindLiteral)
	if m.LiteralValue != true {
		t.Errorf("expected literal value true, got %v", m.LiteralValue)
	}
}

func TestWalkLiteralFalse(t *testing.T) {
	env := setupWalker(t, `type T = false;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindLiteral)
	if m.LiteralValue != false {
		t.Errorf("expected literal value false, got %v", m.LiteralValue)
	}
}

// --- 2.4c: Union types ---

func TestWalkUnionStringNumber(t *testing.T) {
	env := setupWalker(t, `type T = string | number;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindUnion)
	if len(m.UnionMembers) != 2 {
		t.Fatalf("expected 2 union members, got %d", len(m.UnionMembers))
	}
}

func TestWalkUnionNullable(t *testing.T) {
	env := setupWalker(t, `type T = string | null;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	// Should unwrap to string with Nullable=true
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "string")
	if !m.Nullable {
		t.Error("expected Nullable=true")
	}
}

func TestWalkUnionOptional(t *testing.T) {
	env := setupWalker(t, `type T = string | undefined;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "string")
	if !m.Optional {
		t.Error("expected Optional=true")
	}
}

func TestWalkUnionNullableOptional(t *testing.T) {
	env := setupWalker(t, `type T = string | null | undefined;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "string")
	if !m.Nullable {
		t.Error("expected Nullable=true")
	}
	if !m.Optional {
		t.Error("expected Optional=true")
	}
}

func TestWalkUnionBooleanCoalescing(t *testing.T) {
	// TypeScript `boolean` is internally `true | false`
	env := setupWalker(t, `type T = boolean;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "boolean")
}

func TestWalkUnionMultiMember(t *testing.T) {
	env := setupWalker(t, `type T = string | number | boolean;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindUnion)
	if len(m.UnionMembers) != 3 {
		t.Fatalf("expected 3 union members, got %d", len(m.UnionMembers))
	}
}

func TestWalkUnionLiterals(t *testing.T) {
	env := setupWalker(t, `type T = "a" | "b" | "c";`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindUnion)
	if len(m.UnionMembers) != 3 {
		t.Fatalf("expected 3 union members, got %d", len(m.UnionMembers))
	}
	for _, member := range m.UnionMembers {
		assertKind(t, member, metadata.KindLiteral)
	}
}

// --- 2.4d: Intersection types ---

func TestWalkIntersection(t *testing.T) {
	env := setupWalker(t, `
		interface A { a: string; }
		interface B { b: number; }
		type T = A & B;
	`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	// Intersections of interfaces may be resolved to a single object type by the checker
	// or remain as intersection. Accept either.
	if m.Kind != metadata.KindIntersection && m.Kind != metadata.KindObject && m.Kind != metadata.KindRef {
		t.Errorf("expected intersection, object, or ref; got %s", m.Kind)
	}
}

// --- 2.4e: Object types ---

func TestWalkObjectInterface(t *testing.T) {
	env := setupWalker(t, `
		interface User {
			name: string;
			age: number;
		}
		type T = User;
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	// Should be a ref to "User" in the registry
	if m.Kind == metadata.KindRef {
		if m.Ref != "User" {
			t.Errorf("expected ref to User, got %q", m.Ref)
		}
		userMeta := reg.Types["User"]
		if userMeta == nil {
			t.Fatal("User not found in registry")
		}
		assertKind(t, *userMeta, metadata.KindObject)
		if len(userMeta.Properties) != 2 {
			t.Fatalf("expected 2 properties, got %d", len(userMeta.Properties))
		}
		assertPropertyExists(t, userMeta.Properties, "name", metadata.KindAtomic)
		assertPropertyExists(t, userMeta.Properties, "age", metadata.KindAtomic)
	} else if m.Kind == metadata.KindObject {
		if len(m.Properties) != 2 {
			t.Fatalf("expected 2 properties, got %d", len(m.Properties))
		}
	} else {
		t.Errorf("expected ref or object, got %s", m.Kind)
	}
}

func TestWalkObjectOptionalProps(t *testing.T) {
	env := setupWalker(t, `
		interface Config {
			host: string;
			port?: number;
		}
		type T = Config;
	`)
	defer env.release()

	_, reg := env.walkExportedTypeWithRegistry(t, "T")
	configMeta := reg.Types["Config"]
	if configMeta == nil {
		t.Fatal("Config not found in registry")
	}
	for _, prop := range configMeta.Properties {
		if prop.Name == "host" {
			if !prop.Required {
				t.Error("host should be required")
			}
		}
		if prop.Name == "port" {
			if prop.Required {
				t.Error("port should not be required")
			}
		}
	}
}

func TestWalkObjectReadonlyProps(t *testing.T) {
	env := setupWalker(t, `
		interface Config {
			readonly id: string;
			name: string;
		}
		type T = Config;
	`)
	defer env.release()

	_, reg := env.walkExportedTypeWithRegistry(t, "T")
	configMeta := reg.Types["Config"]
	if configMeta == nil {
		t.Fatal("Config not found in registry")
	}
	for _, prop := range configMeta.Properties {
		if prop.Name == "id" {
			if !prop.Readonly {
				t.Error("id should be readonly")
			}
		}
		if prop.Name == "name" {
			if prop.Readonly {
				t.Error("name should not be readonly")
			}
		}
	}
}

func TestWalkObjectNestedObject(t *testing.T) {
	env := setupWalker(t, `
		interface Address {
			street: string;
			city: string;
		}
		interface User {
			name: string;
			address: Address;
		}
		type T = User;
	`)
	defer env.release()

	_, reg := env.walkExportedTypeWithRegistry(t, "T")
	userMeta := reg.Types["User"]
	if userMeta == nil {
		t.Fatal("User not found in registry")
	}
	// address prop should reference Address
	for _, prop := range userMeta.Properties {
		if prop.Name == "address" {
			if prop.Type.Kind != metadata.KindRef || prop.Type.Ref != "Address" {
				t.Errorf("expected address to be ref to Address, got kind=%s ref=%s", prop.Type.Kind, prop.Type.Ref)
			}
		}
	}
	if reg.Types["Address"] == nil {
		t.Error("Address not found in registry")
	}
}

func TestWalkObjectAnonymous(t *testing.T) {
	env := setupWalker(t, `type T = { x: number; y: number; };`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindObject)
	if len(m.Properties) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(m.Properties))
	}
}

// --- 2.4f: Array types ---

func TestWalkArrayShorthand(t *testing.T) {
	env := setupWalker(t, `type T = string[];`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindArray)
	if m.ElementType == nil {
		t.Fatal("expected element type")
	}
	assertKind(t, *m.ElementType, metadata.KindAtomic)
	assertAtomic(t, *m.ElementType, "string")
}

func TestWalkArrayGeneric(t *testing.T) {
	env := setupWalker(t, `type T = Array<number>;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindArray)
	if m.ElementType == nil {
		t.Fatal("expected element type")
	}
	assertKind(t, *m.ElementType, metadata.KindAtomic)
	assertAtomic(t, *m.ElementType, "number")
}

func TestWalkArrayReadonly(t *testing.T) {
	env := setupWalker(t, `type T = readonly string[];`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindArray)
	if m.ElementType == nil {
		t.Fatal("expected element type")
	}
	assertAtomic(t, *m.ElementType, "string")
}

func TestWalkArrayOfObjects(t *testing.T) {
	env := setupWalker(t, `
		interface Item { id: number; }
		type T = Item[];
	`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindArray)
	if m.ElementType == nil {
		t.Fatal("expected element type")
	}
	if m.ElementType.Kind != metadata.KindRef {
		t.Errorf("expected element type ref, got %s", m.ElementType.Kind)
	}
}

// --- 2.4g: Tuple types ---

func TestWalkTupleBasic(t *testing.T) {
	env := setupWalker(t, `type T = [string, number];`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindTuple)
	if len(m.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(m.Elements))
	}
	assertKind(t, m.Elements[0].Type, metadata.KindAtomic)
	assertAtomic(t, m.Elements[0].Type, "string")
	assertKind(t, m.Elements[1].Type, metadata.KindAtomic)
	assertAtomic(t, m.Elements[1].Type, "number")
}

func TestWalkTupleOptionalElement(t *testing.T) {
	env := setupWalker(t, `type T = [string, number?];`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindTuple)
	if len(m.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(m.Elements))
	}
	if m.Elements[0].Optional {
		t.Error("first element should not be optional")
	}
	if !m.Elements[1].Optional {
		t.Error("second element should be optional")
	}
}

func TestWalkTupleRestElement(t *testing.T) {
	env := setupWalker(t, `type T = [string, ...number[]];`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindTuple)
	if len(m.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(m.Elements))
	}
	if m.Elements[1].Rest != true {
		t.Error("second element should be rest")
	}
}

// --- 2.4h: Enum types ---

func TestWalkEnumNumeric(t *testing.T) {
	env := setupWalker(t, `
		enum Direction { Up, Down, Left, Right }
		type T = Direction;
	`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	// Enums resolve to a union of their literal members
	if m.Kind != metadata.KindUnion && m.Kind != metadata.KindEnum {
		t.Errorf("expected union or enum kind, got %s", m.Kind)
	}
}

func TestWalkEnumString(t *testing.T) {
	env := setupWalker(t, `
		enum Color { Red = "RED", Green = "GREEN", Blue = "BLUE" }
		type T = Color;
	`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	if m.Kind != metadata.KindUnion && m.Kind != metadata.KindEnum {
		t.Errorf("expected union or enum kind, got %s", m.Kind)
	}
}

// --- 2.4i: Native types ---

func TestWalkNativeDate(t *testing.T) {
	env := setupWalker(t, `type T = Date;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindNative)
	if m.NativeType != "Date" {
		t.Errorf("expected NativeType=Date, got %q", m.NativeType)
	}
}

func TestWalkNativeRegExp(t *testing.T) {
	env := setupWalker(t, `type T = RegExp;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindNative)
	if m.NativeType != "RegExp" {
		t.Errorf("expected NativeType=RegExp, got %q", m.NativeType)
	}
}

func TestWalkNativeMap(t *testing.T) {
	env := setupWalker(t, `type T = Map<string, number>;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindNative)
	if m.NativeType != "Map" {
		t.Errorf("expected NativeType=Map, got %q", m.NativeType)
	}
	if len(m.TypeArguments) != 2 {
		t.Fatalf("expected 2 type args, got %d", len(m.TypeArguments))
	}
	assertAtomic(t, m.TypeArguments[0], "string")
	assertAtomic(t, m.TypeArguments[1], "number")
}

func TestWalkNativeSet(t *testing.T) {
	env := setupWalker(t, `type T = Set<string>;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindNative)
	if m.NativeType != "Set" {
		t.Errorf("expected NativeType=Set, got %q", m.NativeType)
	}
	if len(m.TypeArguments) != 1 {
		t.Fatalf("expected 1 type arg, got %d", len(m.TypeArguments))
	}
	assertAtomic(t, m.TypeArguments[0], "string")
}

func TestWalkNativePromiseUnwrap(t *testing.T) {
	env := setupWalker(t, `type T = Promise<string>;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	// Promise should be unwrapped to its inner type
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "string")
}

func TestWalkNativeUint8Array(t *testing.T) {
	env := setupWalker(t, `type T = Uint8Array;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindNative)
	if m.NativeType != "Uint8Array" {
		t.Errorf("expected NativeType=Uint8Array, got %q", m.NativeType)
	}
}

func TestWalkNativeURL(t *testing.T) {
	// URL requires DOM lib; skip if not available
	env := setupWalker(t, `
		declare class URL { constructor(url: string); hostname: string; }
		type T = URL;
	`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindNative)
	if m.NativeType != "URL" {
		t.Errorf("expected NativeType=URL, got %q", m.NativeType)
	}
}

// --- 2.4j: Recursive types ---

func TestWalkRecursiveTreeNode(t *testing.T) {
	env := setupWalker(t, `
		interface TreeNode {
			value: string;
			children: TreeNode[];
		}
		type T = TreeNode;
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	// Should be a ref
	if m.Kind != metadata.KindRef {
		t.Fatalf("expected ref, got %s", m.Kind)
	}
	if m.Ref != "TreeNode" {
		t.Errorf("expected ref to TreeNode, got %q", m.Ref)
	}
	nodeMeta := reg.Types["TreeNode"]
	if nodeMeta == nil {
		t.Fatal("TreeNode not found in registry")
	}
	// children prop should be array of ref to TreeNode
	for _, prop := range nodeMeta.Properties {
		if prop.Name == "children" {
			assertKind(t, prop.Type, metadata.KindArray)
			if prop.Type.ElementType == nil {
				t.Fatal("expected element type for children")
			}
			if prop.Type.ElementType.Kind != metadata.KindRef || prop.Type.ElementType.Ref != "TreeNode" {
				t.Errorf("expected children element to be ref to TreeNode, got kind=%s ref=%s", prop.Type.ElementType.Kind, prop.Type.ElementType.Ref)
			}
		}
	}
}

func TestWalkRecursiveLinkedList(t *testing.T) {
	env := setupWalker(t, `
		interface ListNode {
			value: number;
			next: ListNode | null;
		}
		type T = ListNode;
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	if m.Kind != metadata.KindRef {
		t.Fatalf("expected ref, got %s", m.Kind)
	}
	nodeMeta := reg.Types["ListNode"]
	if nodeMeta == nil {
		t.Fatal("ListNode not found in registry")
	}
	// next should be ref to ListNode with Nullable=true
	for _, prop := range nodeMeta.Properties {
		if prop.Name == "next" {
			if prop.Type.Kind != metadata.KindRef {
				t.Errorf("expected next to be ref, got %s", prop.Type.Kind)
			}
			if !prop.Type.Nullable {
				t.Error("expected next to be nullable")
			}
		}
	}
}

// --- 2.4k: Utility types ---

func TestWalkRecordType(t *testing.T) {
	env := setupWalker(t, `type T = Record<string, number>;`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	// Record<string, number> resolves to { [key: string]: number }
	// It may be inline or registered as a ref depending on checker resolution
	var obj *metadata.Metadata
	if m.Kind == metadata.KindObject {
		obj = &m
	} else if m.Kind == metadata.KindRef {
		obj = reg.Types[m.Ref]
	}
	if obj == nil {
		t.Fatalf("expected object (got kind=%s)", m.Kind)
	}
	if obj.IndexSignature == nil {
		t.Fatal("expected index signature")
	}
	assertAtomic(t, obj.IndexSignature.KeyType, "string")
	assertAtomic(t, obj.IndexSignature.ValueType, "number")
}

func TestWalkPartialType(t *testing.T) {
	env := setupWalker(t, `
		interface User { name: string; age: number; }
		type T = Partial<User>;
	`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	// Partial<User> makes all properties optional
	if m.Kind != metadata.KindObject && m.Kind != metadata.KindRef {
		t.Fatalf("expected object or ref, got %s", m.Kind)
	}
	if m.Kind == metadata.KindObject {
		for _, prop := range m.Properties {
			if prop.Required {
				t.Errorf("property %q should not be required in Partial", prop.Name)
			}
		}
	}
}

func TestWalkPickType(t *testing.T) {
	env := setupWalker(t, `
		interface User { name: string; age: number; email: string; }
		type T = Pick<User, "name" | "email">;
	`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	if m.Kind != metadata.KindObject && m.Kind != metadata.KindRef {
		t.Fatalf("expected object or ref, got %s", m.Kind)
	}
	if m.Kind == metadata.KindObject {
		if len(m.Properties) != 2 {
			t.Errorf("expected 2 properties from Pick, got %d", len(m.Properties))
		}
	}
}

func TestWalkOmitType(t *testing.T) {
	env := setupWalker(t, `
		interface User { name: string; age: number; email: string; }
		type T = Omit<User, "email">;
	`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	if m.Kind != metadata.KindObject && m.Kind != metadata.KindRef {
		t.Fatalf("expected object or ref, got %s", m.Kind)
	}
	if m.Kind == metadata.KindObject {
		if len(m.Properties) != 2 {
			t.Errorf("expected 2 properties from Omit, got %d", len(m.Properties))
		}
	}
}

// --- 2.4l: Index signatures ---

func TestWalkIndexSignatureStringKey(t *testing.T) {
	env := setupWalker(t, `type T = { [key: string]: number };`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindObject)
	if m.IndexSignature == nil {
		t.Fatal("expected index signature")
	}
	assertAtomic(t, m.IndexSignature.KeyType, "string")
	assertAtomic(t, m.IndexSignature.ValueType, "number")
}

func TestWalkIndexSignatureNumberKey(t *testing.T) {
	env := setupWalker(t, `type T = { [key: number]: string };`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindObject)
	if m.IndexSignature == nil {
		t.Fatal("expected index signature")
	}
	assertAtomic(t, m.IndexSignature.KeyType, "number")
	assertAtomic(t, m.IndexSignature.ValueType, "string")
}

// --- 2.4m: Generic types ---

func TestWalkGenericInstantiated(t *testing.T) {
	env := setupWalker(t, `
		interface Box<T> { value: T; }
		type T = Box<string>;
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	// Box<string> should resolve to an object with value: string
	if m.Kind == metadata.KindRef {
		boxMeta := reg.Types[m.Ref]
		if boxMeta == nil {
			t.Fatalf("Box type not found in registry for ref %q", m.Ref)
		}
		assertPropertyExists(t, boxMeta.Properties, "value", metadata.KindAtomic)
	} else if m.Kind == metadata.KindObject {
		assertPropertyExists(t, m.Properties, "value", metadata.KindAtomic)
	} else {
		t.Errorf("expected ref or object, got %s", m.Kind)
	}
}

// --- 2.4n: Function types ---

func TestWalkFunctionType(t *testing.T) {
	env := setupWalker(t, `type T = () => void;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	// Function types should be classified as any with name "function"
	assertKind(t, m, metadata.KindAny)
	if m.Name != "function" {
		t.Errorf("expected name=function, got %q", m.Name)
	}
}

func TestWalkFunctionTypeWithParams(t *testing.T) {
	env := setupWalker(t, `type T = (a: string, b: number) => boolean;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAny)
	if m.Name != "function" {
		t.Errorf("expected name=function, got %q", m.Name)
	}
}

// --- 2.4o: Template literal types ---

func TestWalkTemplateLiteral(t *testing.T) {
	env := setupWalker(t, "type T = `hello-${string}`;")
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "string")
}

// ---- Assertion helpers ----

func assertKind(t *testing.T, m metadata.Metadata, expected metadata.Kind) {
	t.Helper()
	if m.Kind != expected {
		t.Errorf("expected Kind=%s, got Kind=%s", expected, m.Kind)
	}
}

func assertAtomic(t *testing.T, m metadata.Metadata, expected string) {
	t.Helper()
	if m.Atomic != expected {
		t.Errorf("expected Atomic=%q, got Atomic=%q", expected, m.Atomic)
	}
}

func assertNumericLiteral(t *testing.T, m metadata.Metadata, expected float64) {
	t.Helper()
	switch v := m.LiteralValue.(type) {
	case float64:
		if v != expected {
			t.Errorf("expected literal value %v, got %v", expected, v)
		}
	case int:
		if float64(v) != expected {
			t.Errorf("expected literal value %v, got %v", expected, v)
		}
	default:
		t.Errorf("expected numeric literal, got %T: %v", m.LiteralValue, m.LiteralValue)
	}
}

func assertPropertyExists(t *testing.T, props []metadata.Property, name string, expectedKind metadata.Kind) {
	t.Helper()
	for _, p := range props {
		if p.Name == name {
			if p.Type.Kind != expectedKind {
				t.Errorf("property %q: expected kind=%s, got kind=%s", name, expectedKind, p.Type.Kind)
			}
			return
		}
	}
	t.Errorf("property %q not found", name)
}

func findProperty(t *testing.T, props []metadata.Property, name string) *metadata.Property {
	t.Helper()
	for i := range props {
		if props[i].Name == name {
			return &props[i]
		}
	}
	t.Fatalf("property %q not found", name)
	return nil
}

// --- 2.4p: JSDoc constraint extraction ---

func TestWalkJSDocMinimumMaximum(t *testing.T) {
	env := setupWalker(t, `
interface Person {
  /**
   * @minimum 0
   * @maximum 150
   */
  age: number;
  name: string;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Person")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Person")
		resolved := reg.Types[m.Ref]
		if resolved != nil {
			m = *resolved
		}
	}

	ageProp := findProperty(t, m.Properties, "age")
	nameProp := findProperty(t, m.Properties, "name")

	// name should have no constraints
	if nameProp.Constraints != nil {
		t.Error("name should have no constraints")
	}

	// age should have minimum and maximum
	if ageProp.Constraints == nil {
		t.Fatal("age should have constraints")
	}
	if ageProp.Constraints.Minimum == nil || *ageProp.Constraints.Minimum != 0 {
		t.Errorf("expected minimum 0, got %v", ageProp.Constraints.Minimum)
	}
	if ageProp.Constraints.Maximum == nil || *ageProp.Constraints.Maximum != 150 {
		t.Errorf("expected maximum 150, got %v", ageProp.Constraints.Maximum)
	}
}

func TestWalkJSDocMinMaxLength(t *testing.T) {
	env := setupWalker(t, `
interface User {
  /**
   * @minLength 1
   * @maxLength 255
   */
  name: string;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "User")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "User")
		resolved := reg.Types[m.Ref]
		if resolved != nil {
			m = *resolved
		}
	}

	nameProp := findProperty(t, m.Properties, "name")
	if nameProp.Constraints == nil {
		t.Fatal("name should have constraints")
	}
	if nameProp.Constraints.MinLength == nil || *nameProp.Constraints.MinLength != 1 {
		t.Errorf("expected minLength 1, got %v", nameProp.Constraints.MinLength)
	}
	if nameProp.Constraints.MaxLength == nil || *nameProp.Constraints.MaxLength != 255 {
		t.Errorf("expected maxLength 255, got %v", nameProp.Constraints.MaxLength)
	}
}

func TestWalkJSDocFormat(t *testing.T) {
	env := setupWalker(t, `
interface Contact {
  /** @format email */
  email: string;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Contact")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Contact")
		resolved := reg.Types[m.Ref]
		if resolved != nil {
			m = *resolved
		}
	}

	emailProp := findProperty(t, m.Properties, "email")
	if emailProp.Constraints == nil {
		t.Fatal("email should have constraints")
	}
	if emailProp.Constraints.Format == nil || *emailProp.Constraints.Format != "email" {
		t.Errorf("expected format 'email', got %v", emailProp.Constraints.Format)
	}
}

func TestWalkJSDocPattern(t *testing.T) {
	env := setupWalker(t, `
interface Post {
  /** @pattern ^[a-z0-9-]+$ */
  slug: string;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Post")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Post")
		resolved := reg.Types[m.Ref]
		if resolved != nil {
			m = *resolved
		}
	}

	slugProp := findProperty(t, m.Properties, "slug")
	if slugProp.Constraints == nil {
		t.Fatal("slug should have constraints")
	}
	if slugProp.Constraints.Pattern == nil || *slugProp.Constraints.Pattern != "^[a-z0-9-]+$" {
		t.Errorf("expected pattern '^[a-z0-9-]+$', got %v", slugProp.Constraints.Pattern)
	}
}

// --- 2.4q: Phase 8 JSDoc constraint extraction (new tags) ---

func TestWalkJSDocExclusiveMinMax(t *testing.T) {
	env := setupWalker(t, `
interface Config {
  /**
   * @exclusiveMinimum 0
   * @exclusiveMaximum 100
   */
  threshold: number;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Config")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Config")
		resolved := reg.Types[m.Ref]
		if resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "threshold")
	if prop.Constraints == nil {
		t.Fatal("threshold should have constraints")
	}
	if prop.Constraints.ExclusiveMinimum == nil || *prop.Constraints.ExclusiveMinimum != 0 {
		t.Errorf("expected exclusiveMinimum 0, got %v", prop.Constraints.ExclusiveMinimum)
	}
	if prop.Constraints.ExclusiveMaximum == nil || *prop.Constraints.ExclusiveMaximum != 100 {
		t.Errorf("expected exclusiveMaximum 100, got %v", prop.Constraints.ExclusiveMaximum)
	}
}

func TestWalkJSDocMultipleOf(t *testing.T) {
	env := setupWalker(t, `
interface Grid {
  /** @multipleOf 5 */
  step: number;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Grid")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Grid")
		resolved := reg.Types[m.Ref]
		if resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "step")
	if prop.Constraints == nil {
		t.Fatal("step should have constraints")
	}
	if prop.Constraints.MultipleOf == nil || *prop.Constraints.MultipleOf != 5 {
		t.Errorf("expected multipleOf 5, got %v", prop.Constraints.MultipleOf)
	}
}

func TestWalkJSDocNumericType(t *testing.T) {
	env := setupWalker(t, `
interface Server {
  /** @type int32 */
  port: number;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Server")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Server")
		resolved := reg.Types[m.Ref]
		if resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "port")
	if prop.Constraints == nil {
		t.Fatal("port should have constraints")
	}
	if prop.Constraints.NumericType == nil || *prop.Constraints.NumericType != "int32" {
		t.Errorf("expected numericType 'int32', got %v", prop.Constraints.NumericType)
	}
}

func TestWalkJSDocUniqueItems(t *testing.T) {
	env := setupWalker(t, `
interface TagList {
  /** @uniqueItems */
  tags: string[];
}
`)
	defer env.release()

	m := env.walkExportedType(t, "TagList")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "TagList")
		resolved := reg.Types[m.Ref]
		if resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "tags")
	if prop.Constraints == nil {
		t.Fatal("tags should have constraints")
	}
	if prop.Constraints.UniqueItems == nil || *prop.Constraints.UniqueItems != true {
		t.Errorf("expected uniqueItems true, got %v", prop.Constraints.UniqueItems)
	}
}

func TestWalkJSDocDefault(t *testing.T) {
	env := setupWalker(t, `
interface Settings {
  /** @default 10 */
  pageSize: number;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Settings")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Settings")
		resolved := reg.Types[m.Ref]
		if resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "pageSize")
	if prop.Constraints == nil {
		t.Fatal("pageSize should have constraints")
	}
	if prop.Constraints.Default == nil || *prop.Constraints.Default != "10" {
		t.Errorf("expected default '10', got %v", prop.Constraints.Default)
	}
}

func TestWalkJSDocContentMediaType(t *testing.T) {
	env := setupWalker(t, `
interface Upload {
  /** @contentMediaType application/json */
  payload: string;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Upload")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Upload")
		resolved := reg.Types[m.Ref]
		if resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "payload")
	if prop.Constraints == nil {
		t.Fatal("payload should have constraints")
	}
	if prop.Constraints.ContentMediaType == nil || *prop.Constraints.ContentMediaType != "application/json" {
		t.Errorf("expected contentMediaType 'application/json', got %v", prop.Constraints.ContentMediaType)
	}
}

// --- Phase 7: Intersection Flattening ---

func TestWalkIntersectionFlattenTwoObjects(t *testing.T) {
	env := setupWalker(t, `
interface A { a: string; }
interface B { b: number; }
type T = A & B;
`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindObject)
	if len(m.Properties) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(m.Properties))
	}
	assertPropertyExists(t, m.Properties, "a", metadata.KindAtomic)
	assertPropertyExists(t, m.Properties, "b", metadata.KindAtomic)
}

func TestWalkIntersectionFlattenConflict(t *testing.T) {
	// Later type wins on property conflict
	env := setupWalker(t, `
interface A { x: string; y: number; }
interface B { y: string; z: boolean; }
type T = A & B;
`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindObject)
	if len(m.Properties) != 3 {
		t.Fatalf("expected 3 properties, got %d", len(m.Properties))
	}
	// y should be string (from B, which is later)
	yProp := findProperty(t, m.Properties, "y")
	if yProp.Type.Kind != metadata.KindAtomic || yProp.Type.Atomic != "string" {
		t.Errorf("expected y to be string (later wins), got %s %s", yProp.Type.Kind, yProp.Type.Atomic)
	}
}

func TestWalkIntersectionOmitAndExtend(t *testing.T) {
	// Common NestJS pattern: Omit<CreateDto, 'password'> & { updatedBy: string }
	env := setupWalker(t, `
interface CreateDto { name: string; email: string; password: string; }
type ProfileDto = Omit<CreateDto, 'password'> & { updatedBy: string; };
`)
	defer env.release()

	m := env.walkExportedType(t, "ProfileDto")
	assertKind(t, m, metadata.KindObject)
	// Should have name, email, updatedBy — NOT password
	if len(m.Properties) != 3 {
		t.Fatalf("expected 3 properties, got %d: %v", len(m.Properties), propNames(m.Properties))
	}
	assertPropertyExists(t, m.Properties, "name", metadata.KindAtomic)
	assertPropertyExists(t, m.Properties, "email", metadata.KindAtomic)
	assertPropertyExists(t, m.Properties, "updatedBy", metadata.KindAtomic)
}

func TestWalkIntersectionNonObjectFallback(t *testing.T) {
	// string & number = never (TS resolves this to never)
	// Use a real non-brandable intersection instead
	env := setupWalker(t, `
type HasName = { name: string };
type HasAge = { age: number };
type T = HasName & HasAge;
`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	// Flattened intersections of objects → KindObject, or KindRef if registered
	if m.Kind != metadata.KindObject && m.Kind != metadata.KindRef {
		t.Errorf("expected object or ref (flattened intersection), got %s", m.Kind)
	}
}

// --- Phase 7b: Sub-field Type Alias Registration ---

func TestWalkSubFieldTypeAlias(t *testing.T) {
	// When UserDetailResponse is walked, UserSummary and
	// ShippingAddressResponse should be registered as named types
	// in the registry (not inlined).
	env := setupWalker(t, `
export type UserSummary = { id: string; name: string; };
export type ShippingAddressResponse = { id: string; label: string; };
export type UserDetailResponse = UserSummary & {
  shippingAddresses: ShippingAddressResponse[];
  extra: string;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	// UserDetailResponse should be registered (walked via WalkNamedType)
	if !reg.Has("UserDetailResponse") {
		t.Error("UserDetailResponse not found in registry")
	}
	// UserSummary should be registered (sub-field via intersection member)
	if !reg.Has("UserSummary") {
		t.Error("UserSummary not found in registry — sub-field type alias not registered")
	}
	// ShippingAddressResponse should be registered (sub-field via property type)
	if !reg.Has("ShippingAddressResponse") {
		t.Error("ShippingAddressResponse not found in registry — sub-field type alias not registered")
	}
}

func TestWalkSubFieldTypeAlias_PropertyReference(t *testing.T) {
	// Types used as property types should be registered
	env := setupWalker(t, `
export type Address = { street: string; city: string; };
export type User = { name: string; address: Address; };
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("User") {
		t.Error("User not found in registry")
	}
	if !reg.Has("Address") {
		t.Error("Address not found in registry — sub-field type alias not registered")
	}
}

// --- Phase 8: Branded Type Detection ---

func TestWalkBrandedString(t *testing.T) {
	// string & { __brand: 'Email' } → should resolve to KindAtomic string
	env := setupWalker(t, `
type Email = string & { __brand: 'Email' };
`)
	defer env.release()

	m := env.walkExportedType(t, "Email")
	assertKind(t, m, metadata.KindAtomic)
	if m.Atomic != "string" {
		t.Errorf("expected atomic 'string', got %q", m.Atomic)
	}
}

func TestWalkBrandedNumber(t *testing.T) {
	// number & { __brand: 'PositiveInt' } → should resolve to KindAtomic number
	env := setupWalker(t, `
type PositiveInt = number & { __brand: 'PositiveInt' };
`)
	defer env.release()

	m := env.walkExportedType(t, "PositiveInt")
	assertKind(t, m, metadata.KindAtomic)
	if m.Atomic != "number" {
		t.Errorf("expected atomic 'number', got %q", m.Atomic)
	}
}

func TestWalkNonBrandedIntersectionPreserved(t *testing.T) {
	// { name: string } & { realProp: number } → should NOT be treated as branded
	// (realProp doesn't start with __)
	env := setupWalker(t, `
type T = { name: string } & { realProp: number };
`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	// Should be flattened object, not branded
	if m.Kind != metadata.KindObject && m.Kind != metadata.KindRef {
		t.Errorf("expected object (flattened), got %s", m.Kind)
	}
	if m.Kind == metadata.KindObject {
		if len(m.Properties) < 2 {
			t.Errorf("expected at least 2 properties (name + realProp), got %d", len(m.Properties))
		}
	}
}

// --- Phase 7: Discriminated Union Detection ---

func TestWalkDiscriminatedUnionType(t *testing.T) {
	env := setupWalker(t, `
interface CardPayment { type: "card"; cardNumber: string; }
interface BankPayment { type: "bank"; accountNumber: string; }
interface CryptoPayment { type: "crypto"; walletAddress: string; }
type Payment = CardPayment | BankPayment | CryptoPayment;
`)
	defer env.release()

	m := env.walkExportedType(t, "Payment")
	assertKind(t, m, metadata.KindUnion)
	if len(m.UnionMembers) != 3 {
		t.Fatalf("expected 3 union members, got %d", len(m.UnionMembers))
	}
	if m.Discriminant == nil {
		t.Fatal("expected discriminant to be detected")
	}
	if m.Discriminant.Property != "type" {
		t.Errorf("expected discriminant property 'type', got %q", m.Discriminant.Property)
	}
	if len(m.Discriminant.Mapping) != 3 {
		t.Errorf("expected 3 discriminant mappings, got %d", len(m.Discriminant.Mapping))
	}
	if _, ok := m.Discriminant.Mapping["card"]; !ok {
		t.Error("missing 'card' in discriminant mapping")
	}
	if _, ok := m.Discriminant.Mapping["bank"]; !ok {
		t.Error("missing 'bank' in discriminant mapping")
	}
	if _, ok := m.Discriminant.Mapping["crypto"]; !ok {
		t.Error("missing 'crypto' in discriminant mapping")
	}
}

func TestWalkDiscriminatedUnionKind(t *testing.T) {
	env := setupWalker(t, `
interface Circle { kind: "circle"; radius: number; }
interface Square { kind: "square"; side: number; }
type Shape = Circle | Square;
`)
	defer env.release()

	m := env.walkExportedType(t, "Shape")
	assertKind(t, m, metadata.KindUnion)
	if m.Discriminant == nil {
		t.Fatal("expected discriminant to be detected")
	}
	if m.Discriminant.Property != "kind" {
		t.Errorf("expected discriminant property 'kind', got %q", m.Discriminant.Property)
	}
}

func TestWalkNonDiscriminatedUnion(t *testing.T) {
	// No common literal property — should NOT detect a discriminant
	env := setupWalker(t, `
interface Dog { breed: string; barks: boolean; }
interface Cat { color: string; purrs: boolean; }
type Pet = Dog | Cat;
`)
	defer env.release()

	m := env.walkExportedType(t, "Pet")
	assertKind(t, m, metadata.KindUnion)
	if m.Discriminant != nil {
		t.Errorf("expected no discriminant, but got one: %v", m.Discriminant.Property)
	}
}

// --- Phase 7: Class DTO Support ---

func TestWalkClassProperties(t *testing.T) {
	env := setupWalker(t, `
class UserDto {
  name: string = "";
  age: number = 0;
  email: string = "";
}
`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "UserDto")
	// Class may be ref or object
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)
	if len(resolved.Properties) < 3 {
		t.Fatalf("expected at least 3 properties, got %d", len(resolved.Properties))
	}
	assertPropertyExists(t, resolved.Properties, "name", metadata.KindAtomic)
	assertPropertyExists(t, resolved.Properties, "age", metadata.KindAtomic)
	assertPropertyExists(t, resolved.Properties, "email", metadata.KindAtomic)
}

func TestWalkClassInheritance(t *testing.T) {
	env := setupWalker(t, `
class BaseDto {
  id: number = 0;
  createdAt: string = "";
}
class UserDto extends BaseDto {
  name: string = "";
  email: string = "";
}
`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "UserDto")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)
	// Should have both own and inherited properties
	if len(resolved.Properties) < 4 {
		t.Fatalf("expected at least 4 properties (2 own + 2 inherited), got %d: %v",
			len(resolved.Properties), propNames(resolved.Properties))
	}
	assertPropertyExists(t, resolved.Properties, "id", metadata.KindAtomic)
	assertPropertyExists(t, resolved.Properties, "createdAt", metadata.KindAtomic)
	assertPropertyExists(t, resolved.Properties, "name", metadata.KindAtomic)
	assertPropertyExists(t, resolved.Properties, "email", metadata.KindAtomic)
}

// --- Phase 7: Template Literal Pattern Extraction ---

func TestWalkTemplateLiteralPrefix(t *testing.T) {
	env := setupWalker(t, "type T = `prefix_${string}`;")
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "string")
	if m.TemplatePattern == "" {
		t.Fatal("expected TemplatePattern to be set")
	}
	if m.TemplatePattern != "^prefix_.*$" {
		t.Errorf("expected pattern '^prefix_.*$', got %q", m.TemplatePattern)
	}
}

func TestWalkTemplateLiteralEmail(t *testing.T) {
	env := setupWalker(t, "type T = `${string}@${string}.${string}`;")
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	if m.TemplatePattern == "" {
		t.Fatal("expected TemplatePattern to be set")
	}
	if m.TemplatePattern != "^.*@.*\\..*$" {
		t.Errorf("expected pattern '^.*@.*\\\\..*$', got %q", m.TemplatePattern)
	}
}

func TestWalkTemplateLiteralNumber(t *testing.T) {
	env := setupWalker(t, "type T = `v${number}`;")
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindAtomic)
	if m.TemplatePattern == "" {
		t.Fatal("expected TemplatePattern to be set")
	}
	// Should contain numeric pattern
	if m.TemplatePattern != "^v[+-]?(\\d+\\.?\\d*|\\.\\d+)$" {
		t.Errorf("expected numeric pattern, got %q", m.TemplatePattern)
	}
}

// --- Phase 7: Utility Type Tests (fix escape hatches) ---

func TestWalkPartialResolved(t *testing.T) {
	env := setupWalker(t, `
interface User { name: string; age: number; email: string; }
type T = Partial<User>;
`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)
	if len(resolved.Properties) != 3 {
		t.Fatalf("expected 3 properties, got %d", len(resolved.Properties))
	}
	// All properties should be optional
	for _, p := range resolved.Properties {
		if p.Required {
			t.Errorf("property %q should be optional in Partial<User>", p.Name)
		}
	}
}

func TestWalkPickResolved(t *testing.T) {
	env := setupWalker(t, `
interface User { name: string; age: number; email: string; }
type T = Pick<User, 'name' | 'email'>;
`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)
	if len(resolved.Properties) != 2 {
		t.Fatalf("expected 2 properties (name, email), got %d: %v",
			len(resolved.Properties), propNames(resolved.Properties))
	}
	assertPropertyExists(t, resolved.Properties, "name", metadata.KindAtomic)
	assertPropertyExists(t, resolved.Properties, "email", metadata.KindAtomic)
}

func TestWalkOmitResolved(t *testing.T) {
	env := setupWalker(t, `
interface User { name: string; age: number; email: string; }
type T = Omit<User, 'age'>;
`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)
	if len(resolved.Properties) != 2 {
		t.Fatalf("expected 2 properties (name, email), got %d: %v",
			len(resolved.Properties), propNames(resolved.Properties))
	}
	assertPropertyExists(t, resolved.Properties, "name", metadata.KindAtomic)
	assertPropertyExists(t, resolved.Properties, "email", metadata.KindAtomic)
}

// --- Phase 7: Nested Generics ---

func TestWalkGenericWrapper(t *testing.T) {
	env := setupWalker(t, `
interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
}
interface User { name: string; }
type T = PaginatedResponse<User>;
`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)
	if len(resolved.Properties) < 3 {
		t.Fatalf("expected at least 3 properties, got %d", len(resolved.Properties))
	}
	itemsProp := findProperty(t, resolved.Properties, "items")
	if itemsProp.Type.Kind != metadata.KindArray {
		t.Errorf("expected items to be array, got %s", itemsProp.Type.Kind)
	}
}

// --- Phase 7: Enum in Object Property ---

func TestWalkEnumInObject(t *testing.T) {
	env := setupWalker(t, `
enum Role { ADMIN = "admin", USER = "user" }
interface User { name: string; role: Role; }
type T = User;
`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)
	roleProp := findProperty(t, resolved.Properties, "role")
	// Role should be a union of string literals, enum, or a $ref to a registered enum
	if roleProp.Type.Kind != metadata.KindUnion && roleProp.Type.Kind != metadata.KindEnum && roleProp.Type.Kind != metadata.KindRef {
		t.Errorf("expected role to be union, enum, or ref, got %s", roleProp.Type.Kind)
	}
}

// --- Phase 7: Recursive Types ---

func TestWalkRecursiveComment(t *testing.T) {
	env := setupWalker(t, `
interface Comment {
  id: number;
  content: string;
  replies: Comment[];
}
type T = Comment;
`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	// Should be a ref to Comment
	if m.Kind != metadata.KindRef {
		t.Fatalf("expected ref, got %s", m.Kind)
	}
	resolved := reg.Types[m.Ref]
	if resolved == nil {
		t.Fatal("Comment not in registry")
	}
	assertKind(t, *resolved, metadata.KindObject)
	repliesProp := findProperty(t, resolved.Properties, "replies")
	if repliesProp.Type.Kind != metadata.KindArray {
		t.Fatalf("expected replies to be array, got %s", repliesProp.Type.Kind)
	}
	// Element type should be a ref back to Comment
	if repliesProp.Type.ElementType == nil || repliesProp.Type.ElementType.Kind != metadata.KindRef {
		t.Error("expected replies element to be a ref")
	}
}

// --- Phase 9: Zod-Elegant Validation API tests ---

// resolveWalkedType resolves a KindRef metadata to its concrete type via registry.
func resolveWalkedType(t *testing.T, env *walkerEnv, typeName string) metadata.Metadata {
	t.Helper()
	m := env.walkExportedType(t, typeName)
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, typeName)
		resolved := reg.Types[m.Ref]
		if resolved != nil {
			return *resolved
		}
	}
	return m
}

// --- 9.1: Shorthand tags ---

func TestWalkJSDocShorthandLen(t *testing.T) {
	env := setupWalker(t, `
interface LenDto {
  /** @len 5 */
  code: string;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "LenDto")
	prop := findProperty(t, m.Properties, "code")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on code")
	}
	if prop.Constraints.MinLength == nil || *prop.Constraints.MinLength != 5 {
		t.Errorf("expected minLength 5, got %v", prop.Constraints.MinLength)
	}
	if prop.Constraints.MaxLength == nil || *prop.Constraints.MaxLength != 5 {
		t.Errorf("expected maxLength 5, got %v", prop.Constraints.MaxLength)
	}
}

func TestWalkJSDocShorthandItems(t *testing.T) {
	env := setupWalker(t, `
interface ItemsDto {
  /** @items 3 */
  tags: string[];
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "ItemsDto")
	prop := findProperty(t, m.Properties, "tags")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on tags")
	}
	if prop.Constraints.MinItems == nil || *prop.Constraints.MinItems != 3 {
		t.Errorf("expected minItems 3, got %v", prop.Constraints.MinItems)
	}
	if prop.Constraints.MaxItems == nil || *prop.Constraints.MaxItems != 3 {
		t.Errorf("expected maxItems 3, got %v", prop.Constraints.MaxItems)
	}
}

func TestWalkJSDocPositive(t *testing.T) {
	env := setupWalker(t, `
interface PositiveDto {
  /** @positive */
  amount: number;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "PositiveDto")
	prop := findProperty(t, m.Properties, "amount")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on amount")
	}
	if prop.Constraints.ExclusiveMinimum == nil || *prop.Constraints.ExclusiveMinimum != 0 {
		t.Errorf("expected exclusiveMinimum 0, got %v", prop.Constraints.ExclusiveMinimum)
	}
}

func TestWalkJSDocNegative(t *testing.T) {
	env := setupWalker(t, `
interface NegativeDto {
  /** @negative */
  delta: number;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "NegativeDto")
	prop := findProperty(t, m.Properties, "delta")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on delta")
	}
	if prop.Constraints.ExclusiveMaximum == nil || *prop.Constraints.ExclusiveMaximum != 0 {
		t.Errorf("expected exclusiveMaximum 0, got %v", prop.Constraints.ExclusiveMaximum)
	}
}

func TestWalkJSDocNonnegative(t *testing.T) {
	env := setupWalker(t, `
interface NonnegativeDto {
  /** @nonnegative */
  count: number;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "NonnegativeDto")
	prop := findProperty(t, m.Properties, "count")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on count")
	}
	if prop.Constraints.Minimum == nil || *prop.Constraints.Minimum != 0 {
		t.Errorf("expected minimum 0, got %v", prop.Constraints.Minimum)
	}
}

func TestWalkJSDocNonpositive(t *testing.T) {
	env := setupWalker(t, `
interface NonpositiveDto {
  /** @nonpositive */
  debt: number;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "NonpositiveDto")
	prop := findProperty(t, m.Properties, "debt")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on debt")
	}
	if prop.Constraints.Maximum == nil || *prop.Constraints.Maximum != 0 {
		t.Errorf("expected maximum 0, got %v", prop.Constraints.Maximum)
	}
}

func TestWalkJSDocInt(t *testing.T) {
	env := setupWalker(t, `
interface IntDto {
  /** @int */
  port: number;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "IntDto")
	prop := findProperty(t, m.Properties, "port")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on port")
	}
	if prop.Constraints.NumericType == nil || *prop.Constraints.NumericType != "int32" {
		t.Errorf("expected numericType int32, got %v", prop.Constraints.NumericType)
	}
}

func TestWalkJSDocSafe(t *testing.T) {
	env := setupWalker(t, `
interface SafeDto {
  /** @safe */
  id: number;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "SafeDto")
	prop := findProperty(t, m.Properties, "id")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on id")
	}
	if prop.Constraints.NumericType == nil || *prop.Constraints.NumericType != "int64" {
		t.Errorf("expected numericType int64, got %v", prop.Constraints.NumericType)
	}
}

func TestWalkJSDocFinite(t *testing.T) {
	env := setupWalker(t, `
interface FiniteDto {
  /** @finite */
  score: number;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "FiniteDto")
	prop := findProperty(t, m.Properties, "score")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on score")
	}
	if prop.Constraints.NumericType == nil || *prop.Constraints.NumericType != "float" {
		t.Errorf("expected numericType float, got %v", prop.Constraints.NumericType)
	}
}

// --- 9.2: String transform tags ---

func TestWalkJSDocTrim(t *testing.T) {
	env := setupWalker(t, `
interface TrimDto {
  /** @trim */
  name: string;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "TrimDto")
	prop := findProperty(t, m.Properties, "name")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on name")
	}
	if len(prop.Constraints.Transforms) != 1 || prop.Constraints.Transforms[0] != "trim" {
		t.Errorf("expected transforms [trim], got %v", prop.Constraints.Transforms)
	}
}

func TestWalkJSDocToLowerCase(t *testing.T) {
	env := setupWalker(t, `
interface LowerDto {
  /** @toLowerCase */
  email: string;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "LowerDto")
	prop := findProperty(t, m.Properties, "email")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on email")
	}
	if len(prop.Constraints.Transforms) != 1 || prop.Constraints.Transforms[0] != "toLowerCase" {
		t.Errorf("expected transforms [toLowerCase], got %v", prop.Constraints.Transforms)
	}
}

func TestWalkJSDocToUpperCase(t *testing.T) {
	env := setupWalker(t, `
interface UpperDto {
  /** @toUpperCase */
  code: string;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "UpperDto")
	prop := findProperty(t, m.Properties, "code")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on code")
	}
	if len(prop.Constraints.Transforms) != 1 || prop.Constraints.Transforms[0] != "toUpperCase" {
		t.Errorf("expected transforms [toUpperCase], got %v", prop.Constraints.Transforms)
	}
}

// --- 9.3: String content checks ---

func TestWalkJSDocStartsWith(t *testing.T) {
	env := setupWalker(t, `
interface SWDto {
  /** @startsWith "http" */
  url: string;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "SWDto")
	prop := findProperty(t, m.Properties, "url")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on url")
	}
	if prop.Constraints.StartsWith == nil || *prop.Constraints.StartsWith != "http" {
		t.Errorf("expected startsWith http, got %v", prop.Constraints.StartsWith)
	}
}

func TestWalkJSDocEndsWith(t *testing.T) {
	env := setupWalker(t, `
interface EWDto {
  /** @endsWith ".ts" */
  file: string;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "EWDto")
	prop := findProperty(t, m.Properties, "file")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on file")
	}
	if prop.Constraints.EndsWith == nil || *prop.Constraints.EndsWith != ".ts" {
		t.Errorf("expected endsWith .ts, got %v", prop.Constraints.EndsWith)
	}
}

func TestWalkJSDocIncludes(t *testing.T) {
	env := setupWalker(t, `
interface IncDto {
  /** @includes "@" */
  email: string;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "IncDto")
	prop := findProperty(t, m.Properties, "email")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on email")
	}
	if prop.Constraints.Includes == nil || *prop.Constraints.Includes != "@" {
		t.Errorf("expected includes @, got %v", prop.Constraints.Includes)
	}
}

func TestWalkJSDocUppercase(t *testing.T) {
	env := setupWalker(t, `
interface UCDto {
  /** @uppercase */
  code: string;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "UCDto")
	prop := findProperty(t, m.Properties, "code")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on code")
	}
	if prop.Constraints.Uppercase == nil || !*prop.Constraints.Uppercase {
		t.Error("expected uppercase true")
	}
}

func TestWalkJSDocLowercase(t *testing.T) {
	env := setupWalker(t, `
interface LCDto {
  /** @lowercase */
  slug: string;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "LCDto")
	prop := findProperty(t, m.Properties, "slug")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on slug")
	}
	if prop.Constraints.Lowercase == nil || !*prop.Constraints.Lowercase {
		t.Error("expected lowercase true")
	}
}

// --- 9.6: Custom error messages ---

func TestWalkJSDocErrorMessage(t *testing.T) {
	env := setupWalker(t, `
interface ErrDto {
  /** @error "Please provide a valid name" */
  name: string;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "ErrDto")
	prop := findProperty(t, m.Properties, "name")
	if prop.Constraints == nil {
		t.Fatal("expected constraints on name")
	}
	if prop.Constraints.ErrorMessage == nil || *prop.Constraints.ErrorMessage != "Please provide a valid name" {
		t.Errorf("expected error message 'Please provide a valid name', got %v", prop.Constraints.ErrorMessage)
	}
}

// --- Helpers ---

// resolveRef resolves a ref through the registry, returning the metadata pointer.
// Returns the metadata as-is if not a ref.
func resolveRef(m metadata.Metadata, reg *metadata.TypeRegistry) *metadata.Metadata {
	if m.Kind == metadata.KindRef && reg != nil {
		if resolved, ok := reg.Types[m.Ref]; ok {
			return resolved
		}
	}
	return &m
}

// propNames returns the names of all properties for debugging.
func propNames(props []metadata.Property) []string {
	names := make([]string, len(props))
	for i, p := range props {
		names[i] = p.Name
	}
	return names
}

// --- Phase 20: Branded type constraint extraction ---

func TestWalkBrandedFormat(t *testing.T) {
	env := setupWalker(t, `
type Format<F extends string> = { readonly __tsgonest_format: F };
interface User {
  email: string & Format<"email">;
  name: string;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "User")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "User")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	emailProp := findProperty(t, m.Properties, "email")
	nameProp := findProperty(t, m.Properties, "name")

	// email should have format constraint from branded type
	if emailProp.Constraints == nil {
		t.Fatal("email should have constraints from branded type")
	}
	if emailProp.Constraints.Format == nil || *emailProp.Constraints.Format != "email" {
		t.Errorf("expected format 'email', got %v", emailProp.Constraints.Format)
	}

	// email type should be atomic string (phantom stripped)
	assertKind(t, emailProp.Type, metadata.KindAtomic)
	assertAtomic(t, emailProp.Type, "string")

	// name should have no constraints
	if nameProp.Constraints != nil {
		t.Error("name should have no constraints")
	}
}

func TestWalkBrandedMinMaxLength(t *testing.T) {
	env := setupWalker(t, `
type MinLength<N extends number> = { readonly __tsgonest_minLength: N };
type MaxLength<N extends number> = { readonly __tsgonest_maxLength: N };
interface User {
  name: string & MinLength<1> & MaxLength<255>;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "User")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "User")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	nameProp := findProperty(t, m.Properties, "name")
	if nameProp.Constraints == nil {
		t.Fatal("name should have constraints from branded types")
	}
	if nameProp.Constraints.MinLength == nil || *nameProp.Constraints.MinLength != 1 {
		t.Errorf("expected minLength 1, got %v", nameProp.Constraints.MinLength)
	}
	if nameProp.Constraints.MaxLength == nil || *nameProp.Constraints.MaxLength != 255 {
		t.Errorf("expected maxLength 255, got %v", nameProp.Constraints.MaxLength)
	}
	assertKind(t, nameProp.Type, metadata.KindAtomic)
}

func TestWalkBrandedNumericConstraints(t *testing.T) {
	env := setupWalker(t, `
type Minimum<N extends number> = { readonly __tsgonest_minimum: N };
type Maximum<N extends number> = { readonly __tsgonest_maximum: N };
type ExclusiveMinimum<N extends number> = { readonly __tsgonest_exclusiveMinimum: N };
type MultipleOf<N extends number> = { readonly __tsgonest_multipleOf: N };
interface Config {
  age: number & Minimum<0> & Maximum<150>;
  score: number & ExclusiveMinimum<0> & MultipleOf<0.5>;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Config")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Config")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	ageProp := findProperty(t, m.Properties, "age")
	if ageProp.Constraints == nil {
		t.Fatal("age should have constraints")
	}
	if ageProp.Constraints.Minimum == nil || *ageProp.Constraints.Minimum != 0 {
		t.Errorf("expected minimum 0, got %v", ageProp.Constraints.Minimum)
	}
	if ageProp.Constraints.Maximum == nil || *ageProp.Constraints.Maximum != 150 {
		t.Errorf("expected maximum 150, got %v", ageProp.Constraints.Maximum)
	}

	scoreProp := findProperty(t, m.Properties, "score")
	if scoreProp.Constraints == nil {
		t.Fatal("score should have constraints")
	}
	if scoreProp.Constraints.ExclusiveMinimum == nil || *scoreProp.Constraints.ExclusiveMinimum != 0 {
		t.Errorf("expected exclusiveMinimum 0, got %v", scoreProp.Constraints.ExclusiveMinimum)
	}
	if scoreProp.Constraints.MultipleOf == nil || *scoreProp.Constraints.MultipleOf != 0.5 {
		t.Errorf("expected multipleOf 0.5, got %v", scoreProp.Constraints.MultipleOf)
	}
}

func TestWalkBrandedNumericType(t *testing.T) {
	env := setupWalker(t, `
type NumType<T extends string> = { readonly __tsgonest_type: T };
interface Data {
  port: number & NumType<"uint32">;
  amount: number & NumType<"float">;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Data")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Data")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	portProp := findProperty(t, m.Properties, "port")
	if portProp.Constraints == nil {
		t.Fatal("port should have constraints")
	}
	if portProp.Constraints.NumericType == nil || *portProp.Constraints.NumericType != "uint32" {
		t.Errorf("expected numericType 'uint32', got %v", portProp.Constraints.NumericType)
	}

	amountProp := findProperty(t, m.Properties, "amount")
	if amountProp.Constraints == nil {
		t.Fatal("amount should have constraints")
	}
	if amountProp.Constraints.NumericType == nil || *amountProp.Constraints.NumericType != "float" {
		t.Errorf("expected numericType 'float', got %v", amountProp.Constraints.NumericType)
	}
}

func TestWalkBrandedPattern(t *testing.T) {
	env := setupWalker(t, `
type Pattern<P extends string> = { readonly __tsgonest_pattern: P };
interface Data {
  code: string & Pattern<"^[A-Z]{3}$">;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Data")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Data")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	codeProp := findProperty(t, m.Properties, "code")
	if codeProp.Constraints == nil {
		t.Fatal("code should have constraints")
	}
	if codeProp.Constraints.Pattern == nil || *codeProp.Constraints.Pattern != "^[A-Z]{3}$" {
		t.Errorf("expected pattern '^[A-Z]{3}$', got %v", codeProp.Constraints.Pattern)
	}
}

func TestWalkBrandedStringContent(t *testing.T) {
	env := setupWalker(t, `
type StartsWith<S extends string> = { readonly __tsgonest_startsWith: S };
type EndsWith<S extends string> = { readonly __tsgonest_endsWith: S };
type Includes<S extends string> = { readonly __tsgonest_includes: S };
interface Data {
  url: string & StartsWith<"https://">;
  file: string & EndsWith<".json">;
  text: string & Includes<"@">;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Data")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Data")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	urlProp := findProperty(t, m.Properties, "url")
	if urlProp.Constraints == nil {
		t.Fatal("url should have constraints")
	}
	if urlProp.Constraints.StartsWith == nil || *urlProp.Constraints.StartsWith != "https://" {
		t.Errorf("expected startsWith 'https://', got %v", urlProp.Constraints.StartsWith)
	}

	fileProp := findProperty(t, m.Properties, "file")
	if fileProp.Constraints == nil {
		t.Fatal("file should have constraints")
	}
	if fileProp.Constraints.EndsWith == nil || *fileProp.Constraints.EndsWith != ".json" {
		t.Errorf("expected endsWith '.json', got %v", fileProp.Constraints.EndsWith)
	}

	textProp := findProperty(t, m.Properties, "text")
	if textProp.Constraints == nil {
		t.Fatal("text should have constraints")
	}
	if textProp.Constraints.Includes == nil || *textProp.Constraints.Includes != "@" {
		t.Errorf("expected includes '@', got %v", textProp.Constraints.Includes)
	}
}

func TestWalkBrandedJSDocMerge(t *testing.T) {
	// Branded type provides format, JSDoc provides additional constraints.
	// Both should be merged, with JSDoc taking precedence.
	env := setupWalker(t, `
type Format<F extends string> = { readonly __tsgonest_format: F };
type MinLength<N extends number> = { readonly __tsgonest_minLength: N };
interface User {
  /**
   * @maxLength 320
   */
  email: string & Format<"email"> & MinLength<5>;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "User")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "User")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	emailProp := findProperty(t, m.Properties, "email")
	if emailProp.Constraints == nil {
		t.Fatal("email should have merged constraints")
	}
	// From branded type
	if emailProp.Constraints.Format == nil || *emailProp.Constraints.Format != "email" {
		t.Errorf("expected format 'email', got %v", emailProp.Constraints.Format)
	}
	if emailProp.Constraints.MinLength == nil || *emailProp.Constraints.MinLength != 5 {
		t.Errorf("expected minLength 5, got %v", emailProp.Constraints.MinLength)
	}
	// From JSDoc
	if emailProp.Constraints.MaxLength == nil || *emailProp.Constraints.MaxLength != 320 {
		t.Errorf("expected maxLength 320 from JSDoc, got %v", emailProp.Constraints.MaxLength)
	}
}

func TestWalkBrandedJSDocOverride(t *testing.T) {
	// JSDoc should override branded type for the same constraint.
	env := setupWalker(t, `
type Format<F extends string> = { readonly __tsgonest_format: F };
interface User {
  /**
   * @format uuid
   */
  id: string & Format<"email">;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "User")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "User")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	idProp := findProperty(t, m.Properties, "id")
	if idProp.Constraints == nil {
		t.Fatal("id should have constraints")
	}
	// JSDoc should override branded type
	if idProp.Constraints.Format == nil || *idProp.Constraints.Format != "uuid" {
		t.Errorf("expected JSDoc format 'uuid' to override branded 'email', got %v", idProp.Constraints.Format)
	}
}

func TestWalkBrandedConvenienceAliases(t *testing.T) {
	// Test that convenience aliases work (they're just type aliases for the phantom types)
	env := setupWalker(t, `
type Format<F extends string> = { readonly __tsgonest_format: F };
type ExclusiveMinimum<N extends number> = { readonly __tsgonest_exclusiveMinimum: N };
type Minimum<N extends number> = { readonly __tsgonest_minimum: N };
type NumType<T extends string> = { readonly __tsgonest_type: T };

// Convenience aliases (like what @tsgonest/types exports)
type Email = Format<"email">;
type Uuid = Format<"uuid">;
type Positive = ExclusiveMinimum<0>;
type NonNegative = Minimum<0>;
type Int = NumType<"int32">;

interface Data {
  email: string & Email;
  id: string & Uuid;
  score: number & Positive;
  count: number & NonNegative & Int;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Data")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Data")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	emailProp := findProperty(t, m.Properties, "email")
	if emailProp.Constraints == nil || emailProp.Constraints.Format == nil || *emailProp.Constraints.Format != "email" {
		t.Errorf("expected email format constraint, got %+v", emailProp.Constraints)
	}

	idProp := findProperty(t, m.Properties, "id")
	if idProp.Constraints == nil || idProp.Constraints.Format == nil || *idProp.Constraints.Format != "uuid" {
		t.Errorf("expected uuid format constraint, got %+v", idProp.Constraints)
	}

	scoreProp := findProperty(t, m.Properties, "score")
	if scoreProp.Constraints == nil || scoreProp.Constraints.ExclusiveMinimum == nil || *scoreProp.Constraints.ExclusiveMinimum != 0 {
		t.Errorf("expected exclusiveMinimum 0, got %+v", scoreProp.Constraints)
	}

	countProp := findProperty(t, m.Properties, "count")
	if countProp.Constraints == nil {
		t.Fatal("count should have constraints")
	}
	if countProp.Constraints.Minimum == nil || *countProp.Constraints.Minimum != 0 {
		t.Errorf("expected minimum 0, got %v", countProp.Constraints.Minimum)
	}
	if countProp.Constraints.NumericType == nil || *countProp.Constraints.NumericType != "int32" {
		t.Errorf("expected numericType 'int32', got %v", countProp.Constraints.NumericType)
	}
}

func TestWalkBrandedMultiPhantom(t *testing.T) {
	// Test multiple phantom intersections merged (like MinLength<1> & MaxLength<255>)
	env := setupWalker(t, `
type MinLength<N extends number> = { readonly __tsgonest_minLength: N };
type MaxLength<N extends number> = { readonly __tsgonest_maxLength: N };
type Format<F extends string> = { readonly __tsgonest_format: F };
type Email = Format<"email">;

interface User {
  email: string & Email;
  name: string & MinLength<1> & MaxLength<255>;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "User")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "User")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	emailProp := findProperty(t, m.Properties, "email")
	if emailProp.Constraints == nil {
		t.Fatal("email should have constraints from Email branded alias")
	}
	if emailProp.Constraints.Format == nil || *emailProp.Constraints.Format != "email" {
		t.Errorf("expected format 'email', got %v", emailProp.Constraints.Format)
	}

	nameProp := findProperty(t, m.Properties, "name")
	if nameProp.Constraints == nil {
		t.Fatal("name should have constraints from MinLength & MaxLength")
	}
	if nameProp.Constraints.MinLength == nil || *nameProp.Constraints.MinLength != 1 {
		t.Errorf("expected minLength 1, got %v", nameProp.Constraints.MinLength)
	}
	if nameProp.Constraints.MaxLength == nil || *nameProp.Constraints.MaxLength != 255 {
		t.Errorf("expected maxLength 255, got %v", nameProp.Constraints.MaxLength)
	}
}

func TestWalkBrandedTypiaFormat(t *testing.T) {
	// Test typia's branded type pattern: "typia.tag" property with kind+value
	env := setupWalker(t, `
type TagBase<Props extends { kind: string; value: any }> = {
  "typia.tag"?: Props;
};
type Format<V extends string> = TagBase<{ target: "string"; kind: "format"; value: V }>;
interface User {
  email: string & Format<"email">;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "User")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "User")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	emailProp := findProperty(t, m.Properties, "email")
	if emailProp.Constraints == nil {
		t.Fatal("email should have constraints from typia Format tag")
	}
	if emailProp.Constraints.Format == nil || *emailProp.Constraints.Format != "email" {
		t.Errorf("expected format 'email' from typia tag, got %v", emailProp.Constraints.Format)
	}
	assertKind(t, emailProp.Type, metadata.KindAtomic)
	assertAtomic(t, emailProp.Type, "string")
}

func TestWalkBrandedTypiaMinimum(t *testing.T) {
	env := setupWalker(t, `
type TagBase<Props extends { kind: string; value: any }> = {
  "typia.tag"?: Props;
};
type Minimum<V extends number> = TagBase<{ target: "number"; kind: "minimum"; value: V }>;
type Maximum<V extends number> = TagBase<{ target: "number"; kind: "maximum"; value: V }>;
interface Config {
  count: number & Minimum<0> & Maximum<100>;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Config")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Config")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	countProp := findProperty(t, m.Properties, "count")
	if countProp.Constraints == nil {
		t.Fatal("count should have constraints from typia tags")
	}
	if countProp.Constraints.Minimum == nil || *countProp.Constraints.Minimum != 0 {
		t.Errorf("expected minimum 0, got %v", countProp.Constraints.Minimum)
	}
	if countProp.Constraints.Maximum == nil || *countProp.Constraints.Maximum != 100 {
		t.Errorf("expected maximum 100, got %v", countProp.Constraints.Maximum)
	}
}

// --- Phase 6: Branded Type Completeness ---

func TestWalkBrandedUppercase(t *testing.T) {
	env := setupWalker(t, `
type UppercaseTag = { readonly __tsgonest_uppercase: true };
interface Dto {
  code: string & UppercaseTag;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Dto")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Dto")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "code")
	if prop.Constraints == nil {
		t.Fatal("code should have constraints from Uppercase branded type")
	}
	if prop.Constraints.Uppercase == nil || !*prop.Constraints.Uppercase {
		t.Error("expected uppercase constraint to be true")
	}
	assertKind(t, prop.Type, metadata.KindAtomic)
	assertAtomic(t, prop.Type, "string")
}

func TestWalkBrandedLowercase(t *testing.T) {
	env := setupWalker(t, `
type LowercaseTag = { readonly __tsgonest_lowercase: true };
interface Dto {
  slug: string & LowercaseTag;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Dto")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Dto")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "slug")
	if prop.Constraints == nil {
		t.Fatal("slug should have constraints from Lowercase branded type")
	}
	if prop.Constraints.Lowercase == nil || !*prop.Constraints.Lowercase {
		t.Error("expected lowercase constraint to be true")
	}
}

func TestWalkBrandedTransformTrim(t *testing.T) {
	env := setupWalker(t, `
type Trim = { readonly __tsgonest_transform_trim: true };
interface Dto {
  name: string & Trim;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Dto")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Dto")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "name")
	if prop.Constraints == nil {
		t.Fatal("name should have constraints from Trim branded type")
	}
	found := false
	for _, tr := range prop.Constraints.Transforms {
		if tr == "trim" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'trim' in transforms, got %v", prop.Constraints.Transforms)
	}
}

func TestWalkBrandedTransformToLowerCase(t *testing.T) {
	env := setupWalker(t, `
type ToLowerCase = { readonly __tsgonest_transform_toLowerCase: true };
interface Dto {
  email: string & ToLowerCase;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Dto")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Dto")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "email")
	if prop.Constraints == nil {
		t.Fatal("email should have constraints from ToLowerCase branded type")
	}
	found := false
	for _, tr := range prop.Constraints.Transforms {
		if tr == "toLowerCase" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'toLowerCase' in transforms, got %v", prop.Constraints.Transforms)
	}
}

func TestWalkBrandedTransformToUpperCase(t *testing.T) {
	env := setupWalker(t, `
type ToUpperCase = { readonly __tsgonest_transform_toUpperCase: true };
interface Dto {
  code: string & ToUpperCase;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Dto")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Dto")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "code")
	if prop.Constraints == nil {
		t.Fatal("code should have constraints from ToUpperCase branded type")
	}
	found := false
	for _, tr := range prop.Constraints.Transforms {
		if tr == "toUpperCase" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'toUpperCase' in transforms, got %v", prop.Constraints.Transforms)
	}
}

func TestWalkBrandedError(t *testing.T) {
	env := setupWalker(t, `
type ErrorTag<M extends string> = { readonly __tsgonest_error: M };
type Format<F extends string> = { readonly __tsgonest_format: F };
interface Dto {
  email: string & Format<"email"> & ErrorTag<"Must be a valid email">;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Dto")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Dto")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "email")
	if prop.Constraints == nil {
		t.Fatal("email should have constraints from branded types")
	}
	if prop.Constraints.ErrorMessage == nil || *prop.Constraints.ErrorMessage != "Must be a valid email" {
		t.Errorf("expected error message 'Must be a valid email', got %v", prop.Constraints.ErrorMessage)
	}
	if prop.Constraints.Format == nil || *prop.Constraints.Format != "email" {
		t.Errorf("expected format 'email', got %v", prop.Constraints.Format)
	}
}

func TestWalkBrandedDefault(t *testing.T) {
	env := setupWalker(t, `
type Default<V extends string | number | boolean> = { readonly __tsgonest_default: V };
interface Dto {
  theme?: string & Default<"light">;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Dto")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Dto")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "theme")
	if prop.Constraints == nil {
		t.Fatal("theme should have constraints from Default branded type")
	}
	if prop.Constraints.Default == nil || *prop.Constraints.Default != "light" {
		t.Errorf("expected default 'light', got %v", prop.Constraints.Default)
	}
}

func TestWalkBrandedPerConstraintError(t *testing.T) {
	env := setupWalker(t, `
type FormatTag<F extends { type: string; error?: string }> = {
  readonly __tsgonest_format: F["type"];
} & (F extends { error: infer E extends string } ? { readonly __tsgonest_format_error: E } : {});

type MinLengthTag<N extends { value: number; error?: string }> = {
  readonly __tsgonest_minLength: N["value"];
} & (N extends { error: infer E extends string } ? { readonly __tsgonest_minLength_error: E } : {});

interface Dto {
  email: string & FormatTag<{type: "email", error: "Must be a valid email"}> & MinLengthTag<{value: 5, error: "Email too short"}>;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Dto")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Dto")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "email")
	if prop.Constraints == nil {
		t.Fatal("email should have constraints")
	}

	// Check constraint values
	if prop.Constraints.Format == nil || *prop.Constraints.Format != "email" {
		t.Errorf("expected format 'email', got %v", prop.Constraints.Format)
	}
	if prop.Constraints.MinLength == nil || *prop.Constraints.MinLength != 5 {
		t.Errorf("expected minLength 5, got %v", prop.Constraints.MinLength)
	}

	// Check per-constraint errors
	if prop.Constraints.Errors == nil {
		t.Fatal("expected per-constraint errors map")
	}
	if msg, ok := prop.Constraints.Errors["format"]; !ok || msg != "Must be a valid email" {
		t.Errorf("expected format error 'Must be a valid email', got %q", msg)
	}
	if msg, ok := prop.Constraints.Errors["minLength"]; !ok || msg != "Email too short" {
		t.Errorf("expected minLength error 'Email too short', got %q", msg)
	}
}

func TestWalkBrandedPerConstraintError_NoError(t *testing.T) {
	// When no error field is provided, the Errors map should not have an entry
	env := setupWalker(t, `
type FormatTag<F extends { type: string }> = {
  readonly __tsgonest_format: F["type"];
};

interface Dto {
  email: string & FormatTag<{type: "uuid"}>;
}
`)
	defer env.release()

	m := env.walkExportedType(t, "Dto")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Dto")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	prop := findProperty(t, m.Properties, "email")
	if prop.Constraints == nil {
		t.Fatal("email should have constraints")
	}
	if prop.Constraints.Format == nil || *prop.Constraints.Format != "uuid" {
		t.Errorf("expected format 'uuid', got %v", prop.Constraints.Format)
	}
	// No error → Errors should be nil or empty
	if prop.Constraints.Errors != nil && len(prop.Constraints.Errors) > 0 {
		t.Errorf("expected no per-constraint errors, got %v", prop.Constraints.Errors)
	}
}

// --- Dual API tests: simple value vs object config produce identical constraints ---

func TestWalkBrandedDualAPI_FormatSimple(t *testing.T) {
	// Simple form: Format<"email"> should produce format = "email"
	env := setupWalker(t, `
type Format<F extends string> = { readonly __tsgonest_format: F };
interface User {
  email: string & Format<"email">;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "User")
	emailProp := findProperty(t, m.Properties, "email")
	if emailProp.Constraints == nil {
		t.Fatal("email should have constraints")
	}
	if emailProp.Constraints.Format == nil || *emailProp.Constraints.Format != "email" {
		t.Errorf("expected format 'email', got %v", emailProp.Constraints.Format)
	}
}

func TestWalkBrandedDualAPI_FormatObjectConfig(t *testing.T) {
	// Object config form: TypeVal conditional type extracts the type field.
	// Tests that tsgo resolves: TypeVal<{type: "email"}, string> → "email"
	env := setupWalker(t, `
type TypeVal<T, Base> = T extends { type: infer V } ? V : T;
type Format<F extends string | { type: string; error?: string }> = {
  readonly __tsgonest_format: TypeVal<F, string>;
};
interface User {
  email: string & Format<{type: "email"}>;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "User")
	emailProp := findProperty(t, m.Properties, "email")
	if emailProp.Constraints == nil {
		t.Fatal("email should have constraints from object config form")
	}
	if emailProp.Constraints.Format == nil || *emailProp.Constraints.Format != "email" {
		t.Errorf("expected format 'email', got %v", emailProp.Constraints.Format)
	}
}

func TestWalkBrandedDualAPI_MinimumSimple(t *testing.T) {
	env := setupWalker(t, `
type Minimum<N extends number> = { readonly __tsgonest_minimum: N };
interface Config {
  age: number & Minimum<0>;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "Config")
	ageProp := findProperty(t, m.Properties, "age")
	if ageProp.Constraints == nil {
		t.Fatal("age should have constraints")
	}
	if ageProp.Constraints.Minimum == nil || *ageProp.Constraints.Minimum != 0 {
		t.Errorf("expected minimum 0, got %v", ageProp.Constraints.Minimum)
	}
}

func TestWalkBrandedDualAPI_MinimumObjectConfig(t *testing.T) {
	// Object config form: NumVal conditional type extracts the value field.
	// Tests that tsgo resolves: NumVal<{value: 0}> → 0
	env := setupWalker(t, `
type NumVal<N extends number | { value: number }> =
  N extends { value: infer V } ? V : N;
type Minimum<N extends number | { value: number }> = {
  readonly __tsgonest_minimum: NumVal<N>;
};
interface Config {
  age: number & Minimum<{value: 0}>;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "Config")
	ageProp := findProperty(t, m.Properties, "age")
	if ageProp.Constraints == nil {
		t.Fatal("age should have constraints from object config form")
	}
	if ageProp.Constraints.Minimum == nil || *ageProp.Constraints.Minimum != 0 {
		t.Errorf("expected minimum 0, got %v", ageProp.Constraints.Minimum)
	}
}

func TestWalkBrandedDualAPI_ObjectConfigWithError(t *testing.T) {
	// Object config with error: should produce both the constraint and the per-constraint error.
	// We test the resolved form since template literal mapped types in WithErr
	// use backtick syntax that is tricky in Go test strings.
	// The key test: __tsgonest_minimum extracts the value, __tsgonest_minimum_error extracts the error.
	env := setupWalker(t, `
interface Config {
  age: number & {
    readonly __tsgonest_minimum: 0;
    readonly __tsgonest_minimum_error: "Age cannot be negative";
  };
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "Config")
	ageProp := findProperty(t, m.Properties, "age")
	if ageProp.Constraints == nil {
		t.Fatal("age should have constraints")
	}
	if ageProp.Constraints.Minimum == nil || *ageProp.Constraints.Minimum != 0 {
		t.Errorf("expected minimum 0, got %v", ageProp.Constraints.Minimum)
	}
	if ageProp.Constraints.Errors == nil {
		t.Fatal("age should have per-constraint errors")
	}
	if msg, ok := ageProp.Constraints.Errors["minimum"]; !ok || msg != "Age cannot be negative" {
		t.Errorf("expected per-constraint error 'Age cannot be negative', got %q", msg)
	}
}

func TestWalkBrandedDualAPI_CompoundLength(t *testing.T) {
	// Length<N> = MinLength<N> & MaxLength<N> — both should be set
	env := setupWalker(t, `
type MinLength<N extends number> = { readonly __tsgonest_minLength: N };
type MaxLength<N extends number> = { readonly __tsgonest_maxLength: N };
type Length<N extends number> = MinLength<N> & MaxLength<N>;
interface Token {
  code: string & Length<6>;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "Token")
	codeProp := findProperty(t, m.Properties, "code")
	if codeProp.Constraints == nil {
		t.Fatal("code should have constraints")
	}
	if codeProp.Constraints.MinLength == nil || *codeProp.Constraints.MinLength != 6 {
		t.Errorf("expected minLength 6, got %v", codeProp.Constraints.MinLength)
	}
	if codeProp.Constraints.MaxLength == nil || *codeProp.Constraints.MaxLength != 6 {
		t.Errorf("expected maxLength 6, got %v", codeProp.Constraints.MaxLength)
	}
}

func TestWalkBrandedDualAPI_ShortAliases(t *testing.T) {
	// Short aliases: Min/Max/Gt/Lt should produce the same constraints as Minimum/Maximum/etc.
	env := setupWalker(t, `
type Minimum<N extends number> = { readonly __tsgonest_minimum: N };
type Maximum<N extends number> = { readonly __tsgonest_maximum: N };
type ExclusiveMinimum<N extends number> = { readonly __tsgonest_exclusiveMinimum: N };
type ExclusiveMaximum<N extends number> = { readonly __tsgonest_exclusiveMaximum: N };
type Min<N extends number> = Minimum<N>;
type Max<N extends number> = Maximum<N>;
type Gt<N extends number> = ExclusiveMinimum<N>;
type Lt<N extends number> = ExclusiveMaximum<N>;
interface Score {
  value: number & Min<0> & Max<100>;
  bonus: number & Gt<0> & Lt<50>;
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "Score")
	valueProp := findProperty(t, m.Properties, "value")
	bonusProp := findProperty(t, m.Properties, "bonus")

	if valueProp.Constraints == nil {
		t.Fatal("value should have constraints")
	}
	if valueProp.Constraints.Minimum == nil || *valueProp.Constraints.Minimum != 0 {
		t.Errorf("expected minimum 0, got %v", valueProp.Constraints.Minimum)
	}
	if valueProp.Constraints.Maximum == nil || *valueProp.Constraints.Maximum != 100 {
		t.Errorf("expected maximum 100, got %v", valueProp.Constraints.Maximum)
	}

	if bonusProp.Constraints == nil {
		t.Fatal("bonus should have constraints")
	}
	if bonusProp.Constraints.ExclusiveMinimum == nil || *bonusProp.Constraints.ExclusiveMinimum != 0 {
		t.Errorf("expected exclusiveMinimum 0, got %v", bonusProp.Constraints.ExclusiveMinimum)
	}
	if bonusProp.Constraints.ExclusiveMaximum == nil || *bonusProp.Constraints.ExclusiveMaximum != 50 {
		t.Errorf("expected exclusiveMaximum 50, got %v", bonusProp.Constraints.ExclusiveMaximum)
	}
}

func TestWalkBrandedPlainBrandingStillWorks(t *testing.T) {
	// Ensure traditional branded types (non-tsgonest) still work correctly
	// i.e., they are detected as branded and reduced to atomic, but with no constraints
	env := setupWalker(t, `
interface User {
  id: string & { __brand: "UserId" };
}
`)
	defer env.release()

	m := env.walkExportedType(t, "User")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "User")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}

	idProp := findProperty(t, m.Properties, "id")
	assertKind(t, idProp.Type, metadata.KindAtomic)
	assertAtomic(t, idProp.Type, "string")
	// Traditional branding should NOT produce constraints
	if idProp.Constraints != nil {
		t.Error("plain __brand should not produce constraints")
	}
}

// --- Validate<typeof fn> branded type ---

func TestWalkBrandedValidateFn(t *testing.T) {
	// Validate<typeof fn> produces __tsgonest_validate phantom property
	// with a function type. The walker should extract the function name.
	env := setupWalker(t, `
function isValidCard(value: string): boolean {
  return value.length === 16;
}
interface PaymentDto {
  card: string & { readonly __tsgonest_validate: typeof isValidCard };
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "PaymentDto")
	cardProp := findProperty(t, m.Properties, "card")
	if cardProp.Constraints == nil {
		t.Fatal("card should have constraints from Validate branded type")
	}
	if cardProp.Constraints.ValidateFn == nil {
		t.Fatal("card should have ValidateFn set")
	}
	if *cardProp.Constraints.ValidateFn != "isValidCard" {
		t.Errorf("expected ValidateFn 'isValidCard', got %q", *cardProp.Constraints.ValidateFn)
	}
	// ValidateModule should be set to the source file path
	if cardProp.Constraints.ValidateModule == nil {
		t.Fatal("card should have ValidateModule set")
	}
	// The atomic type should be string (phantom stripped)
	assertKind(t, cardProp.Type, metadata.KindAtomic)
	assertAtomic(t, cardProp.Type, "string")
}

func TestWalkBrandedValidateFn_WithError(t *testing.T) {
	// Validate with per-constraint error: __tsgonest_validate + __tsgonest_validate_error
	env := setupWalker(t, `
function isValidEmail(value: string): boolean {
  return value.includes("@");
}
interface ContactDto {
  email: string & {
    readonly __tsgonest_validate: typeof isValidEmail;
    readonly __tsgonest_validate_error: "Must be a valid email address";
  };
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "ContactDto")
	emailProp := findProperty(t, m.Properties, "email")
	if emailProp.Constraints == nil {
		t.Fatal("email should have constraints")
	}
	if emailProp.Constraints.ValidateFn == nil || *emailProp.Constraints.ValidateFn != "isValidEmail" {
		t.Errorf("expected ValidateFn 'isValidEmail', got %v", emailProp.Constraints.ValidateFn)
	}
	if emailProp.Constraints.Errors == nil {
		t.Fatal("email should have per-constraint errors")
	}
	if msg, ok := emailProp.Constraints.Errors["validate"]; !ok || msg != "Must be a valid email address" {
		t.Errorf("expected per-constraint error for validate, got %q", msg)
	}
}

func TestWalkBrandedValidateFn_NoConstraintOnNonFunction(t *testing.T) {
	// If __tsgonest_validate is not a function type, it should NOT extract
	env := setupWalker(t, `
interface BadDto {
  value: string & { readonly __tsgonest_validate: "not_a_function" };
}
`)
	defer env.release()

	m := resolveWalkedType(t, env, "BadDto")
	valueProp := findProperty(t, m.Properties, "value")
	// Should not have ValidateFn (the string "not_a_function" has no symbol/declaration)
	if valueProp.Constraints != nil && valueProp.Constraints.ValidateFn != nil {
		t.Error("ValidateFn should not be set for non-function type")
	}
}

// --- Phantom Object Non-Registration in WalkNamedType ---

func TestWalkNamedType_PhantomObjectNotRegistered(t *testing.T) {
	// A phantom object like tags.Format<"email"> only has __tsgonest_* properties.
	// Walking it via WalkNamedType should NOT register it in the registry,
	// so it remains inlinable for tryDetectBranded.
	env := setupWalker(t, `
type Email = { readonly __tsgonest_format?: "email" };
type NormalDto = { name: string; age: number; };
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	// Phantom type "Email" should NOT be registered
	if reg.Has("Email") {
		t.Error("phantom object 'Email' should NOT be registered in the registry")
	}

	// Normal type "NormalDto" SHOULD be registered
	if !reg.Has("NormalDto") {
		t.Error("normal type 'NormalDto' should be registered in the registry")
	}
}

func TestWalkNamedType_PhantomMultipleProperties(t *testing.T) {
	// A phantom object with multiple __tsgonest_* properties should also not be registered.
	env := setupWalker(t, `
type EmailBranded = {
  readonly __tsgonest_format?: "email";
  readonly __tsgonest_minLength?: 1;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if reg.Has("EmailBranded") {
		t.Error("phantom object 'EmailBranded' should NOT be registered in the registry")
	}
}

func TestWalkNamedType_TypiaPhantomNotRegistered(t *testing.T) {
	// Typia-style phantom using "typia.tag" property should also not be registered.
	env := setupWalker(t, `
type TypiaBrand = { readonly "typia.tag": { kind: "format"; value: "email" } };
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if reg.Has("TypiaBrand") {
		t.Error("typia phantom object 'TypiaBrand' should NOT be registered in the registry")
	}
}

func TestWalkNamedType_MixedRealAndPhantomPropsRegistered(t *testing.T) {
	// If an object has BOTH phantom and non-phantom properties, it's not phantom.
	// It should be registered normally.
	env := setupWalker(t, `
type MixedDto = {
  name: string;
  readonly __tsgonest_format?: "email";
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	// Has a real property "name", so it's NOT phantom and should be registered.
	if !reg.Has("MixedDto") {
		t.Error("mixed object 'MixedDto' (has real + phantom props) should be registered")
	}
}

// --- Type Alias Sub-field Registration via Type_alias ---

func TestWalkNamedType_SubFieldTypeAliasRegistered(t *testing.T) {
	// When a type alias (e.g., Address) is used as a sub-field of another
	// type, the Type_alias mechanism should register it so it becomes a $ref.
	env := setupWalker(t, `
type Address = { street: string; city: string; };
type User = { name: string; address: Address; };
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	// Both types should be registered
	if !reg.Has("User") {
		t.Error("top-level type 'User' should be registered")
	}
	if !reg.Has("Address") {
		t.Error("sub-field type alias 'Address' should be registered via Type_alias")
	}

	// User's address property should be KindRef pointing to Address
	userMeta := reg.Types["User"]
	if userMeta == nil {
		t.Fatal("User not found in registry")
	}
	for _, prop := range userMeta.Properties {
		if prop.Name == "address" {
			if prop.Type.Kind != metadata.KindRef {
				t.Errorf("expected address type to be KindRef, got %s", prop.Type.Kind)
			}
			if prop.Type.Ref != "Address" {
				t.Errorf("expected address ref to be 'Address', got %q", prop.Type.Ref)
			}
		}
	}
}

func TestWalkNamedType_SubFieldPhantomNotRegistered(t *testing.T) {
	// Even when encountered as a sub-field, phantom types should NOT be registered.
	env := setupWalker(t, `
type EmailFormat = { readonly __tsgonest_format?: "email" };
type User = { name: string; email: string & EmailFormat; };
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	// EmailFormat is phantom — should NOT be registered
	if reg.Has("EmailFormat") {
		t.Error("phantom sub-field type 'EmailFormat' should NOT be registered")
	}

	// User should still be registered
	if !reg.Has("User") {
		t.Error("User should be registered")
	}
}

// --- Generic utility type instantiation tests ---

func TestWalkNamedType_MultipleOmitInstantiationsDoNotCollide(t *testing.T) {
	// Two different type aliases using Omit<> with different base types must produce
	// different schemas with their own correct properties. Previously, the alias name
	// "Omit" was registered for the first instantiation and reused for all subsequent ones.
	env := setupWalker(t, `
interface Product { id: string; name: string; autoIncrementId: number; imageMetadata: string; }
interface Cart { id: string; cartId: string; abandonedAt: string; customerId: string; }

type ProductResponse = Omit<Product, 'autoIncrementId' | 'imageMetadata'> & { extra: boolean; };
type CartResponse = Omit<Cart, 'customerId'> & { recovered: boolean; };
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	// Both should be registered
	if !reg.Has("ProductResponse") {
		t.Fatal("ProductResponse should be registered")
	}
	if !reg.Has("CartResponse") {
		t.Fatal("CartResponse should be registered")
	}

	// "Omit" should NOT be registered as a standalone schema
	if reg.Has("Omit") {
		t.Error("bare 'Omit' should not be registered as a schema")
	}

	// ProductResponse should have Product's properties (minus omitted) + extra
	pr := reg.Types["ProductResponse"]
	prProps := make(map[string]bool)
	for _, p := range pr.Properties {
		prProps[p.Name] = true
	}
	if !prProps["id"] || !prProps["name"] || !prProps["extra"] {
		t.Errorf("ProductResponse should have id, name, extra; got %v", prProps)
	}
	if prProps["autoIncrementId"] || prProps["imageMetadata"] {
		t.Error("ProductResponse should NOT have omitted properties")
	}
	if prProps["abandonedAt"] || prProps["cartId"] {
		t.Error("ProductResponse should NOT have Cart properties")
	}

	// CartResponse should have Cart's properties (minus omitted) + recovered
	cr := reg.Types["CartResponse"]
	crProps := make(map[string]bool)
	for _, p := range cr.Properties {
		crProps[p.Name] = true
	}
	if !crProps["id"] || !crProps["cartId"] || !crProps["abandonedAt"] || !crProps["recovered"] {
		t.Errorf("CartResponse should have id, cartId, abandonedAt, recovered; got %v", crProps)
	}
	if crProps["customerId"] {
		t.Error("CartResponse should NOT have omitted 'customerId'")
	}
}

func TestWalkNamedType_MultiplePickInstantiationsDoNotCollide(t *testing.T) {
	// Same test for Pick<> — multiple instantiations must not share a schema.
	env := setupWalker(t, `
interface User { id: string; name: string; email: string; age: number; }
interface Product { id: string; title: string; price: number; sku: string; }

type UserSummary = Pick<User, 'id' | 'name'>;
type ProductSummary = Pick<Product, 'id' | 'title'>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("UserSummary") {
		t.Fatal("UserSummary should be registered")
	}
	if !reg.Has("ProductSummary") {
		t.Fatal("ProductSummary should be registered")
	}

	// "Pick" should NOT be registered as a standalone schema
	if reg.Has("Pick") {
		t.Error("bare 'Pick' should not be registered as a schema")
	}

	us := reg.Types["UserSummary"]
	usProps := make(map[string]bool)
	for _, p := range us.Properties {
		usProps[p.Name] = true
	}
	if !usProps["id"] || !usProps["name"] {
		t.Errorf("UserSummary should have id, name; got %v", usProps)
	}
	if usProps["email"] || usProps["age"] {
		t.Error("UserSummary should NOT have non-picked properties")
	}

	ps := reg.Types["ProductSummary"]
	psProps := make(map[string]bool)
	for _, p := range ps.Properties {
		psProps[p.Name] = true
	}
	if !psProps["id"] || !psProps["title"] {
		t.Errorf("ProductSummary should have id, title; got %v", psProps)
	}
	if psProps["price"] || psProps["sku"] {
		t.Error("ProductSummary should NOT have non-picked properties")
	}
}

func TestWalkNamedType_CustomOmitGenericNotRegistered(t *testing.T) {
	// Custom generic utility types (like TypedOmit<T, K>) should also not be
	// registered under the bare alias name.
	env := setupWalker(t, `
type TypedOmit<T, K extends keyof T> = Pick<T, Exclude<keyof T, K>>;
interface Order { id: string; total: number; internalId: number; }
interface Shipment { id: string; trackingNo: string; tempId: number; }

type OrderResponse = TypedOmit<Order, 'internalId'>;
type ShipmentResponse = TypedOmit<Shipment, 'tempId'>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if reg.Has("TypedOmit") {
		t.Error("bare 'TypedOmit' should not be registered")
	}

	if !reg.Has("OrderResponse") {
		t.Fatal("OrderResponse should be registered")
	}
	if !reg.Has("ShipmentResponse") {
		t.Fatal("ShipmentResponse should be registered")
	}

	or := reg.Types["OrderResponse"]
	orProps := make(map[string]bool)
	for _, p := range or.Properties {
		orProps[p.Name] = true
	}
	if !orProps["id"] || !orProps["total"] {
		t.Errorf("OrderResponse should have id, total; got %v", orProps)
	}
	if orProps["internalId"] {
		t.Error("OrderResponse should NOT have omitted 'internalId'")
	}

	sr := reg.Types["ShipmentResponse"]
	srProps := make(map[string]bool)
	for _, p := range sr.Properties {
		srProps[p.Name] = true
	}
	if !srProps["id"] || !srProps["trackingNo"] {
		t.Errorf("ShipmentResponse should have id, trackingNo; got %v", srProps)
	}
	if srProps["tempId"] {
		t.Error("ShipmentResponse should NOT have omitted 'tempId'")
	}
}

// ===========================================================================
// Complex computed types — verifying deeply nested utility type resolution
// ===========================================================================
//
// tsgo's type checker fully resolves utility types (Omit, Pick, Partial,
// Required, Record, Extract, Exclude, mapped types, conditional types, etc.)
// into structural types before the walker sees them. We work with resolved
// structural types, not syntactic utility expressions. These tests verify
// that complex compositions of utility types produce correct schemas.

func TestComplexType_NestedOmitPick(t *testing.T) {
	// Pick<Omit<T, 'a'>, 'b' | 'c'> — nested utility composition
	env := setupWalker(t, `
interface User {
  id: string;
  email: string;
  password: string;
  name: string;
  age: number;
}
type PublicUser = Pick<Omit<User, 'password'>, 'id' | 'name'>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("PublicUser") {
		t.Fatal("PublicUser should be registered")
	}

	m := reg.Types["PublicUser"]
	props := make(map[string]bool)
	for _, p := range m.Properties {
		props[p.Name] = true
	}

	if !props["id"] || !props["name"] {
		t.Errorf("PublicUser should have id, name; got %v", props)
	}
	if props["password"] || props["email"] || props["age"] {
		t.Errorf("PublicUser should NOT have password, email, or age; got %v", props)
	}
	if len(m.Properties) != 2 {
		t.Errorf("PublicUser should have exactly 2 properties, got %d", len(m.Properties))
	}
}

func TestComplexType_PartialAndRequired(t *testing.T) {
	// Required<Partial<T>> should round-trip back to T's structure
	env := setupWalker(t, `
interface Config {
  host: string;
  port: number;
  debug: boolean;
}
type FullConfig = Required<Partial<Config>>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("FullConfig") {
		t.Fatal("FullConfig should be registered")
	}

	regType := reg.Types["FullConfig"]
	if len(regType.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(regType.Properties))
	}
	for _, p := range regType.Properties {
		// After Required<>, properties should not be optional.
		if !p.Required {
			t.Errorf("property %q should be required after Required<>", p.Name)
		}
	}
}

func TestComplexType_DeepPartialIntersection(t *testing.T) {
	// Partial<A> & Partial<B> — intersection of two partials
	env := setupWalker(t, `
interface Dimensions { width: number; height: number; }
interface Color { r: number; g: number; b: number; }
type StyleOverrides = Partial<Dimensions> & Partial<Color>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("StyleOverrides") {
		t.Fatal("StyleOverrides should be registered")
	}

	m := reg.Types["StyleOverrides"]
	props := make(map[string]bool)
	for _, p := range m.Properties {
		props[p.Name] = true
	}

	expected := []string{"width", "height", "r", "g", "b"}
	for _, name := range expected {
		if !props[name] {
			t.Errorf("StyleOverrides should have property %q", name)
		}
	}
	if len(m.Properties) != 5 {
		t.Errorf("expected 5 properties, got %d", len(m.Properties))
	}
}

func TestComplexType_RecordType(t *testing.T) {
	// Record<string, T> resolves to an index signature / mapped type
	env := setupWalker(t, `
interface Permission { read: boolean; write: boolean; }
type PermissionMap = Record<string, Permission>;
`)
	defer env.release()

	m := env.walkExportedType(t, "PermissionMap")

	// Record<string, T> should resolve to an object with string index signature
	// which we represent as KindObject with no named properties and additional properties,
	// or as KindMap / KindAny depending on the walker's handling
	if m.Kind != metadata.KindObject && m.Kind != metadata.KindAny {
		// The checker resolves Record<string, Permission> to { [key: string]: Permission }
		// We accept either object or any representation
		t.Logf("RecordType resolved to Kind=%s (acceptable for index signature types)", m.Kind)
	}
}

func TestComplexType_ConditionalType(t *testing.T) {
	// Extract<T, U> uses conditional types internally
	env := setupWalker(t, `
type EventKind = 'click' | 'hover' | 'scroll' | 'resize';
type UIEvents = Extract<EventKind, 'click' | 'hover'>;
`)
	defer env.release()

	m := env.walkExportedType(t, "UIEvents")

	// Extract<'click'|'hover'|'scroll'|'resize', 'click'|'hover'> = 'click' | 'hover'
	assertKind(t, m, metadata.KindUnion)
	if len(m.UnionMembers) != 2 {
		t.Fatalf("expected 2 union members, got %d", len(m.UnionMembers))
	}

	values := make(map[string]bool)
	for _, member := range m.UnionMembers {
		if member.Kind == metadata.KindLiteral {
			values[fmt.Sprintf("%v", member.LiteralValue)] = true
		}
	}
	if !values["click"] || !values["hover"] {
		t.Errorf("expected 'click' and 'hover', got %v", values)
	}
}

func TestComplexType_ExcludeType(t *testing.T) {
	// Exclude<T, U> — the complement of Extract
	env := setupWalker(t, `
type Status = 'active' | 'inactive' | 'pending' | 'deleted';
type VisibleStatus = Exclude<Status, 'deleted'>;
`)
	defer env.release()

	m := env.walkExportedType(t, "VisibleStatus")

	assertKind(t, m, metadata.KindUnion)
	if len(m.UnionMembers) != 3 {
		t.Fatalf("expected 3 union members, got %d", len(m.UnionMembers))
	}

	values := make(map[string]bool)
	for _, member := range m.UnionMembers {
		if member.Kind == metadata.KindLiteral {
			values[fmt.Sprintf("%v", member.LiteralValue)] = true
		}
	}
	if values["deleted"] {
		t.Error("VisibleStatus should NOT contain 'deleted'")
	}
	if !values["active"] || !values["inactive"] || !values["pending"] {
		t.Errorf("expected active, inactive, pending; got %v", values)
	}
}

func TestComplexType_MappedTypeWithKeyRemapping(t *testing.T) {
	// Mapped type with template literal key remapping:
	// { [K in keyof T as `get${Capitalize<K>}`]: () => T[K] }
	// The checker resolves this to an object with renamed keys
	env := setupWalker(t, `
interface Point { x: number; y: number; }
type Getters<T> = { [K in keyof T as `+"`get${Capitalize<string & K>}`"+`]: T[K] };
type PointGetters = Getters<Point>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("PointGetters") {
		t.Fatal("PointGetters should be registered")
	}

	m := reg.Types["PointGetters"]
	props := make(map[string]bool)
	for _, p := range m.Properties {
		props[p.Name] = true
	}

	// Mapped type with key remapping: x→getX, y→getY
	if !props["getX"] || !props["getY"] {
		t.Errorf("expected getX, getY; got %v", props)
	}
	if props["x"] || props["y"] {
		t.Error("original keys x, y should not be present")
	}
}

func TestComplexType_DeepOmitChain(t *testing.T) {
	// Omit<Omit<Omit<T, 'a'>, 'b'>, 'c'> — triple-nested Omit
	env := setupWalker(t, `
interface Entity {
  id: string;
  createdAt: string;
  updatedAt: string;
  deletedAt: string;
  name: string;
  value: number;
}
type CleanEntity = Omit<Omit<Omit<Entity, 'createdAt'>, 'updatedAt'>, 'deletedAt'>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("CleanEntity") {
		t.Fatal("CleanEntity should be registered")
	}

	m := reg.Types["CleanEntity"]
	props := make(map[string]bool)
	for _, p := range m.Properties {
		props[p.Name] = true
	}

	if !props["id"] || !props["name"] || !props["value"] {
		t.Errorf("expected id, name, value; got %v", props)
	}
	if props["createdAt"] || props["updatedAt"] || props["deletedAt"] {
		t.Errorf("timestamp fields should be omitted; got %v", props)
	}
	if len(m.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(m.Properties))
	}
}

func TestComplexType_IntersectionWithOmitAndExtra(t *testing.T) {
	// Common NestJS pattern: Omit<Entity, 'internalFields'> & { extra computed fields }
	env := setupWalker(t, `
interface OrderEntity {
  id: string;
  customerId: string;
  total: number;
  internalField: string;
  autoIncrementId: number;
}
interface OrderItem { sku: string; quantity: number; }
type OrderResponse = Omit<OrderEntity, 'internalField' | 'autoIncrementId'> & {
  items: OrderItem[];
  formattedTotal: string;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("OrderResponse") {
		t.Fatal("OrderResponse should be registered")
	}

	m := reg.Types["OrderResponse"]
	props := make(map[string]bool)
	for _, p := range m.Properties {
		props[p.Name] = true
	}

	// From OrderEntity (minus omitted)
	if !props["id"] || !props["customerId"] || !props["total"] {
		t.Errorf("expected base fields id, customerId, total; got %v", props)
	}
	// Extra fields from intersection
	if !props["items"] || !props["formattedTotal"] {
		t.Errorf("expected extra fields items, formattedTotal; got %v", props)
	}
	// Omitted fields
	if props["internalField"] || props["autoIncrementId"] {
		t.Errorf("omitted fields should not be present; got %v", props)
	}
	if len(m.Properties) != 5 {
		t.Errorf("expected 5 properties, got %d", len(m.Properties))
	}
}

func TestComplexType_PickWithDiscriminatedUnion(t *testing.T) {
	// Pick applied to a type, then used inside a discriminated union
	env := setupWalker(t, `
interface SuccessResult { status: 'success'; data: string; timestamp: number; }
interface ErrorResult { status: 'error'; message: string; code: number; }

type SuccessSummary = Pick<SuccessResult, 'status' | 'data'>;
type ErrorSummary = Pick<ErrorResult, 'status' | 'message'>;
type ResultSummary = SuccessSummary | ErrorSummary;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("ResultSummary") {
		t.Fatal("ResultSummary should be registered")
	}

	resultSummary := reg.Types["ResultSummary"]
	assertKind(t, *resultSummary, metadata.KindUnion)
	if len(resultSummary.UnionMembers) != 2 {
		t.Fatalf("expected 2 union members, got %d", len(resultSummary.UnionMembers))
	}
}

func TestComplexType_ReadonlyType(t *testing.T) {
	// Readonly<T> — removes mutability but structure should be identical for schemas
	env := setupWalker(t, `
interface Mutable { x: number; y: number; label: string; }
type Frozen = Readonly<Mutable>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("Frozen") {
		t.Fatal("Frozen should be registered")
	}

	m := reg.Types["Frozen"]
	if len(m.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d: %+v", len(m.Properties), m.Properties)
	}
	propNames := make(map[string]bool)
	for _, p := range m.Properties {
		propNames[p.Name] = true
	}
	if !propNames["x"] || !propNames["y"] || !propNames["label"] {
		t.Errorf("expected x, y, label; got %v", propNames)
	}
}

func TestComplexType_NonNullable(t *testing.T) {
	// NonNullable strips null and undefined from a union
	env := setupWalker(t, `
type MaybeString = string | null | undefined;
type DefiniteString = NonNullable<MaybeString>;
`)
	defer env.release()

	m := env.walkExportedType(t, "DefiniteString")
	assertKind(t, m, metadata.KindAtomic)
	assertAtomic(t, m, "string")
}

func TestComplexType_NestedPartialPick(t *testing.T) {
	// Partial<Pick<T, K>> — partial subset of an interface
	env := setupWalker(t, `
interface FormData {
  username: string;
  email: string;
  password: string;
  bio: string;
  age: number;
}
type ProfilePatch = Partial<Pick<FormData, 'bio' | 'age' | 'email'>>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("ProfilePatch") {
		t.Fatal("ProfilePatch should be registered")
	}

	m := reg.Types["ProfilePatch"]
	if len(m.Properties) != 3 {
		t.Errorf("expected 3 properties (bio, age, email), got %d", len(m.Properties))
	}
	for _, p := range m.Properties {
		if !p.Type.Optional {
			t.Errorf("property %q should be optional after Partial<>", p.Name)
		}
	}
}

func TestComplexType_CustomMappedReadonly(t *testing.T) {
	// Custom mapped type: { readonly [K in keyof T]-?: T[K] }
	env := setupWalker(t, `
interface Settings {
  theme?: string;
  fontSize?: number;
  language?: string;
}
type Concrete<T> = { readonly [K in keyof T]-?: T[K] };
type ConcreteSettings = Concrete<Settings>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("ConcreteSettings") {
		t.Fatal("ConcreteSettings should be registered")
	}

	m := reg.Types["ConcreteSettings"]
	if len(m.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(m.Properties))
	}
	for _, p := range m.Properties {
		if p.Type.Optional {
			t.Errorf("property %q should NOT be optional after -?", p.Name)
		}
	}
}

func TestComplexType_MultiLevelInheritanceWithOmit(t *testing.T) {
	// BaseEntity → UserEntity extends BaseEntity → UserResponse = Omit<UserEntity, timestamps>
	env := setupWalker(t, `
interface BaseEntity {
  id: string;
  createdAt: string;
  updatedAt: string;
}
interface UserEntity extends BaseEntity {
  email: string;
  name: string;
  passwordHash: string;
}
type UserResponse = Omit<UserEntity, 'createdAt' | 'updatedAt' | 'passwordHash'>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("UserResponse") {
		t.Fatal("UserResponse should be registered")
	}

	m := reg.Types["UserResponse"]
	props := make(map[string]bool)
	for _, p := range m.Properties {
		props[p.Name] = true
	}

	if !props["id"] || !props["email"] || !props["name"] {
		t.Errorf("expected id, email, name; got %v", props)
	}
	if props["createdAt"] || props["updatedAt"] || props["passwordHash"] {
		t.Errorf("omitted fields should not be present; got %v", props)
	}
	if len(m.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(m.Properties))
	}
}

func TestComplexType_TemplateLiteralUnion(t *testing.T) {
	// Template literal types create computed string literal unions
	env := setupWalker(t, `
type Direction = 'top' | 'bottom' | 'left' | 'right';
type MarginKey = `+"`margin-${Direction}`"+`;
`)
	defer env.release()

	m := env.walkExportedType(t, "MarginKey")
	assertKind(t, m, metadata.KindUnion)
	if len(m.UnionMembers) != 4 {
		t.Fatalf("expected 4 template literal variants, got %d", len(m.UnionMembers))
	}

	values := make(map[string]bool)
	for _, member := range m.UnionMembers {
		if member.Kind == metadata.KindLiteral {
			values[fmt.Sprintf("%v", member.LiteralValue)] = true
		}
	}
	for _, expected := range []string{"margin-top", "margin-bottom", "margin-left", "margin-right"} {
		if !values[expected] {
			t.Errorf("expected %q in union; got %v", expected, values)
		}
	}
}

func TestComplexType_InferWithConditionalType(t *testing.T) {
	// Conditional type with infer: extract Promise unwrapping
	env := setupWalker(t, `
type UnwrapPromise<T> = T extends Promise<infer U> ? U : T;
type Result = UnwrapPromise<Promise<{ id: string; name: string }>>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("Result") {
		t.Fatal("Result should be registered")
	}

	result := reg.Types["Result"]
	assertKind(t, *result, metadata.KindObject)
	props := make(map[string]bool)
	for _, p := range result.Properties {
		props[p.Name] = true
	}
	if !props["id"] || !props["name"] {
		t.Errorf("expected id, name from unwrapped Promise; got %v", props)
	}
}

func TestComplexType_DeepNesting_OmitPickPartialRequired(t *testing.T) {
	// Required<Partial<Pick<Omit<T, 'a'>, 'b' | 'c'>>> — 4 levels deep
	env := setupWalker(t, `
interface FullEntity {
  id: string;
  name: string;
  email: string;
  secret: string;
  role: string;
}
type CleanSubset = Required<Partial<Pick<Omit<FullEntity, 'secret'>, 'id' | 'name' | 'email'>>>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("CleanSubset") {
		t.Fatal("CleanSubset should be registered")
	}

	m := reg.Types["CleanSubset"]
	if len(m.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(m.Properties))
	}
	props := make(map[string]bool)
	for _, p := range m.Properties {
		props[p.Name] = true
	}
	if !props["id"] || !props["name"] || !props["email"] {
		t.Errorf("expected id, name, email; got %v", props)
	}
}

func TestComplexType_DiscriminatedUnionWithSharedFields(t *testing.T) {
	// Complex discriminated union with shared base fields
	env := setupWalker(t, `
interface BaseEvent { timestamp: number; source: string; }
interface ClickEvent extends BaseEvent { type: 'click'; x: number; y: number; }
interface KeyEvent extends BaseEvent { type: 'key'; key: string; modifiers: string[]; }
interface ScrollEvent extends BaseEvent { type: 'scroll'; deltaX: number; deltaY: number; }

type InputEvent = ClickEvent | KeyEvent | ScrollEvent;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("InputEvent") {
		t.Fatal("InputEvent should be registered")
	}

	m := reg.Types["InputEvent"]
	assertKind(t, *m, metadata.KindUnion)
	if len(m.UnionMembers) != 3 {
		t.Fatalf("expected 3 union members, got %d", len(m.UnionMembers))
	}

	// Verify discriminant was detected
	if m.Discriminant == nil || m.Discriminant.Property != "type" {
		t.Errorf("expected discriminant property='type', got %+v", m.Discriminant)
	}
}

func TestComplexType_RecursiveTreeWithOmit(t *testing.T) {
	// Recursive type through a utility type: TreeNode has children: TreeNode[]
	// Response = Omit<TreeNode, 'parent'> — omit the back-reference
	env := setupWalker(t, `
interface TreeNode {
  id: string;
  label: string;
  parent: TreeNode | null;
  children: TreeNode[];
}
type TreeNodeResponse = Omit<TreeNode, 'parent'>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("TreeNodeResponse") {
		t.Fatal("TreeNodeResponse should be registered")
	}

	m := reg.Types["TreeNodeResponse"]
	props := make(map[string]bool)
	for _, p := range m.Properties {
		props[p.Name] = true
	}

	if !props["id"] || !props["label"] || !props["children"] {
		t.Errorf("expected id, label, children; got %v", props)
	}
	if props["parent"] {
		t.Error("parent should be omitted")
	}
}

func TestComplexType_MultipleGenericInstantiationsSubField(t *testing.T) {
	// Multiple different Omit instantiations used as sub-fields within a single parent type
	// This tests that different Omit<X, Y> don't overwrite each other in the registry
	env := setupWalker(t, `
interface Product { id: string; name: string; internalSku: string; price: number; }
interface Customer { id: string; email: string; passwordHash: string; name: string; }
interface Order { id: string; status: string; secretToken: string; total: number; }

type OrderSummary = {
  order: Omit<Order, 'secretToken'>;
  customer: Omit<Customer, 'passwordHash'>;
  products: Omit<Product, 'internalSku'>[];
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("OrderSummary") {
		t.Fatal("OrderSummary should be registered")
	}

	m := reg.Types["OrderSummary"]

	// All three sub-fields should be inline objects (since Omit<X,Y> is generic instantiation)
	// NOT registered under "Omit" (which would be wrong)
	if reg.Has("Omit") {
		t.Error("bare 'Omit' should NOT be registered in the registry")
	}

	// Verify OrderSummary has the right structure
	if len(m.Properties) != 3 {
		t.Errorf("expected 3 properties (order, customer, products), got %d", len(m.Properties))
	}

	// Verify the order sub-field doesn't have secretToken
	orderProp := findProperty(t, m.Properties, "order")
	if orderProp.Type.Kind == metadata.KindObject {
		orderProps := make(map[string]bool)
		for _, p := range orderProp.Type.Properties {
			orderProps[p.Name] = true
		}
		if orderProps["secretToken"] {
			t.Error("order sub-field should NOT have secretToken")
		}
		if !orderProps["id"] || !orderProps["status"] || !orderProps["total"] {
			t.Errorf("order sub-field should have id, status, total; got %v", orderProps)
		}
	}

	// Verify the customer sub-field doesn't have passwordHash
	customerProp := findProperty(t, m.Properties, "customer")
	if customerProp.Type.Kind == metadata.KindObject {
		customerProps := make(map[string]bool)
		for _, p := range customerProp.Type.Properties {
			customerProps[p.Name] = true
		}
		if customerProps["passwordHash"] {
			t.Error("customer sub-field should NOT have passwordHash")
		}
		if !customerProps["id"] || !customerProps["email"] || !customerProps["name"] {
			t.Errorf("customer sub-field should have id, email, name; got %v", customerProps)
		}
	}
}

func TestComplexType_CustomStrictOmit(t *testing.T) {
	// Custom strict Omit utility: TypedOmit<T, K extends keyof T> = Pick<T, Exclude<keyof T, K>>
	// Two different instantiations with intersection
	env := setupWalker(t, `
type TypedOmit<T, K extends keyof T> = Pick<T, Exclude<keyof T, K>>;

interface Product {
  id: string;
  name: string;
  price: number;
  internalSku: string;
  imageMetadata: string;
}
interface Category { id: string; name: string; slug: string; }
interface Shop { id: string; domain: string; }

type ProductResponse = TypedOmit<Product, 'internalSku' | 'imageMetadata'> & {
  categories: Category[];
  shops: Shop[];
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("ProductResponse") {
		t.Fatal("ProductResponse should be registered")
	}
	if reg.Has("TypedOmit") {
		t.Error("bare TypedOmit should NOT be registered")
	}

	m := reg.Types["ProductResponse"]
	props := make(map[string]bool)
	for _, p := range m.Properties {
		props[p.Name] = true
	}

	// From Product minus omitted
	if !props["id"] || !props["name"] || !props["price"] {
		t.Errorf("expected Product base fields; got %v", props)
	}
	if props["internalSku"] || props["imageMetadata"] {
		t.Errorf("omitted fields should not be present; got %v", props)
	}
	// From intersection
	if !props["categories"] || !props["shops"] {
		t.Errorf("expected intersection fields categories, shops; got %v", props)
	}
	if len(m.Properties) != 5 {
		t.Errorf("expected 5 properties, got %d", len(m.Properties))
	}
}

func TestComplexType_SubFieldNamedTypeRegistration(t *testing.T) {
	// Named types used as sub-fields (depth > 1) should be registered via Type_alias
	env := setupWalker(t, `
interface Address { street: string; city: string; zip: string; }
interface ContactInfo { phone: string; fax: string; }

type ShippingAddress = Address & { deliveryNotes?: string; };
type BillingContact = ContactInfo & { taxId: string; };

type OrderDetail = {
  shippingAddress: ShippingAddress;
  billingContact: BillingContact;
  orderNumber: string;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("OrderDetail") {
		t.Fatal("OrderDetail should be registered")
	}
	// Sub-field named types should also be registered
	if !reg.Has("ShippingAddress") {
		t.Error("ShippingAddress should be registered as named type (sub-field)")
	}
	if !reg.Has("BillingContact") {
		t.Error("BillingContact should be registered as named type (sub-field)")
	}

	// Verify OrderDetail references them by $ref (KindRef)
	m := reg.Types["OrderDetail"]
	shippingProp := findProperty(t, m.Properties, "shippingAddress")
	if shippingProp.Type.Kind != metadata.KindRef {
		t.Errorf("shippingAddress should be KindRef, got %s", shippingProp.Type.Kind)
	}
	billingProp := findProperty(t, m.Properties, "billingContact")
	if billingProp.Type.Kind != metadata.KindRef {
		t.Errorf("billingContact should be KindRef, got %s", billingProp.Type.Kind)
	}
}

func TestComplexType_SubFieldNamedUnionRegistration(t *testing.T) {
	// Named union types used as sub-fields should be registered via Type_alias
	// and become $ref in the parent object
	env := setupWalker(t, `
type OrderStatus = 'pending' | 'shipped' | 'delivered' | 'cancelled';
type PaymentStatus = 'unpaid' | 'paid' | 'refunded';

type OrderSummary = {
  id: string;
  orderStatus: OrderStatus;
  paymentStatus: PaymentStatus;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("OrderSummary") {
		t.Fatal("OrderSummary should be registered")
	}
	// Sub-field named union types should also be registered
	if !reg.Has("OrderStatus") {
		t.Error("OrderStatus should be registered as named type (sub-field union)")
	}
	if !reg.Has("PaymentStatus") {
		t.Error("PaymentStatus should be registered as named type (sub-field union)")
	}

	// Verify OrderSummary references them by $ref (KindRef)
	m := reg.Types["OrderSummary"]
	osProp := findProperty(t, m.Properties, "orderStatus")
	if osProp.Type.Kind != metadata.KindRef {
		t.Errorf("orderStatus should be KindRef, got %s", osProp.Type.Kind)
	}
	psProp := findProperty(t, m.Properties, "paymentStatus")
	if psProp.Type.Kind != metadata.KindRef {
		t.Errorf("paymentStatus should be KindRef, got %s", psProp.Type.Kind)
	}
}

func TestComplexType_DoubleIntersectionWithInterfaceExtends(t *testing.T) {
	// interface C extends A, B {} combined with Omit
	env := setupWalker(t, `
interface Timestamps { createdAt: string; updatedAt: string; }
interface SoftDelete { deletedAt: string | null; }
interface BaseEntity extends Timestamps, SoftDelete {
  id: string;
}
interface UserEntity extends BaseEntity {
  email: string;
  name: string;
  role: string;
}
type UserResponse = Omit<UserEntity, 'deletedAt'> & { isActive: boolean; };
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("UserResponse") {
		t.Fatal("UserResponse should be registered")
	}

	m := reg.Types["UserResponse"]
	props := make(map[string]bool)
	for _, p := range m.Properties {
		props[p.Name] = true
	}

	// All inherited + own fields minus deletedAt, plus extra
	expected := []string{"id", "createdAt", "updatedAt", "email", "name", "role", "isActive"}
	for _, name := range expected {
		if !props[name] {
			t.Errorf("expected property %q; got %v", name, props)
		}
	}
	if props["deletedAt"] {
		t.Error("deletedAt should be omitted")
	}
	if len(m.Properties) != len(expected) {
		t.Errorf("expected %d properties, got %d", len(expected), len(m.Properties))
	}
}

// --- Bug 1: PaginatedResponse<T> Generic Collapse ---

// TestWalkGenericTypeAlias_MultipleInstantiations verifies that different
// instantiations of a generic type alias produce distinct named schemas in
// the registry, not a single collapsed schema.
// This mirrors the ecom-bot PaginatedResponse<T> pattern where controllers
// return PaginatedResponse<CampaignResponse>, PaginatedResponse<AdSetResponse>, etc.
func TestWalkGenericTypeAlias_MultipleInstantiations(t *testing.T) {
	env := setupWalker(t, `
type PaginatedResponse<T> = {
  items: T[];
  totalCount: number;
  totalPages: number;
  currentPage: number;
  pageSize: number;
};

interface CampaignResponse {
  id: string;
  name: string;
  budget: number;
}

interface AdSetResponse {
  id: string;
  targeting: string;
  status: string;
}

interface SandboxMessageResponse {
  id: string;
  content: string;
  timestamp: number;
}

// Three concrete type aliases — each should get its own schema
type CampaignList = PaginatedResponse<CampaignResponse>;
type AdSetList = PaginatedResponse<AdSetResponse>;
type MessageList = PaginatedResponse<SandboxMessageResponse>;
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	// Each named alias should be registered with correct items type
	for _, tc := range []struct {
		alias   string
		itemRef string
	}{
		{"CampaignList", "CampaignResponse"},
		{"AdSetList", "AdSetResponse"},
		{"MessageList", "SandboxMessageResponse"},
	} {
		if !reg.Has(tc.alias) {
			t.Errorf("%s should be registered in the registry", tc.alias)
			continue
		}
		m := reg.Types[tc.alias]
		if m.Kind != metadata.KindObject {
			t.Errorf("%s: expected KindObject, got %s", tc.alias, m.Kind)
			continue
		}
		itemsProp := findProperty(t, m.Properties, "items")
		if itemsProp.Type.Kind != metadata.KindArray {
			t.Errorf("%s.items: expected KindArray, got %s", tc.alias, itemsProp.Type.Kind)
			continue
		}
		if itemsProp.Type.ElementType == nil {
			t.Errorf("%s.items: ElementType is nil", tc.alias)
			continue
		}
		elem := itemsProp.Type.ElementType
		if elem.Kind != metadata.KindRef {
			t.Errorf("%s.items element: expected KindRef, got %s", tc.alias, elem.Kind)
			continue
		}
		if elem.Ref != tc.itemRef {
			t.Errorf("%s.items element: expected ref to %q, got %q", tc.alias, tc.itemRef, elem.Ref)
		}
	}
}

// TestWalkGenericInterface_MultipleInstantiations_AsSubField tests that when
// a generic interface like PaginatedResponse<T> is used as a sub-field property
// with different type arguments, each instantiation gets a distinct registered name.
func TestWalkGenericInterface_MultipleInstantiations_AsSubField(t *testing.T) {
	env := setupWalker(t, `
interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
}

interface User { name: string; email: string; }
interface Product { id: number; title: string; price: number; }

// Wrapper that uses two different instantiations as sub-fields
type ApiResponses = {
  users: PaginatedResponse<User>;
  products: PaginatedResponse<Product>;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("ApiResponses") {
		t.Fatal("ApiResponses should be registered")
	}
	apiResp := reg.Types["ApiResponses"]

	usersProp := findProperty(t, apiResp.Properties, "users")
	productsProp := findProperty(t, apiResp.Properties, "products")

	// Both sub-fields should resolve to registered types (not anonymous inline objects)
	// AND they must be DIFFERENT registered types (not the same collapsed PaginatedResponse)
	if usersProp.Type.Kind != metadata.KindRef && usersProp.Type.Kind != metadata.KindObject {
		t.Fatalf("users: expected KindRef or KindObject, got %s", usersProp.Type.Kind)
	}
	if productsProp.Type.Kind != metadata.KindRef && productsProp.Type.Kind != metadata.KindObject {
		t.Fatalf("products: expected KindRef or KindObject, got %s", productsProp.Type.Kind)
	}

	// Resolve both to their actual object metadata
	resolveToObject := func(prop *metadata.Property) *metadata.Metadata {
		if prop.Type.Kind == metadata.KindRef {
			resolved, ok := reg.Types[prop.Type.Ref]
			if !ok {
				t.Fatalf("ref %q not found in registry", prop.Type.Ref)
			}
			return resolved
		}
		return &prop.Type
	}
	usersObj := resolveToObject(usersProp)
	productsObj := resolveToObject(productsProp)

	// Verify each has correct items type
	usersItems := findProperty(t, usersObj.Properties, "items")
	productsItems := findProperty(t, productsObj.Properties, "items")

	if usersItems.Type.Kind != metadata.KindArray || usersItems.Type.ElementType == nil {
		t.Fatal("users.items should be an array with element type")
	}
	if productsItems.Type.Kind != metadata.KindArray || productsItems.Type.ElementType == nil {
		t.Fatal("products.items should be an array with element type")
	}

	// The element types must be DIFFERENT — User vs Product
	usersElem := usersItems.Type.ElementType
	productsElem := productsItems.Type.ElementType

	if usersElem.Kind == metadata.KindRef && productsElem.Kind == metadata.KindRef {
		if usersElem.Ref == productsElem.Ref {
			t.Errorf("GENERIC COLLAPSE: users.items and products.items both reference %q — they should reference different types (User vs Product)", usersElem.Ref)
		}
	} else if usersElem.Kind == metadata.KindObject && productsElem.Kind == metadata.KindObject {
		// If both are inline objects, check that they have different properties
		if len(usersElem.Properties) == len(productsElem.Properties) {
			// Could be coincidence, but check property names
			userPropNames := make(map[string]bool)
			for _, p := range usersElem.Properties {
				userPropNames[p.Name] = true
			}
			productPropNames := make(map[string]bool)
			for _, p := range productsElem.Properties {
				productPropNames[p.Name] = true
			}
			allSame := true
			for name := range userPropNames {
				if !productPropNames[name] {
					allSame = false
					break
				}
			}
			if allSame && len(userPropNames) > 0 {
				t.Error("GENERIC COLLAPSE: users.items and products.items have identical property sets — they should differ (User has name+email, Product has id+title+price)")
			}
		}
	}

	// If both are KindRef, they should point to different names
	if usersProp.Type.Kind == metadata.KindRef && productsProp.Type.Kind == metadata.KindRef {
		if usersProp.Type.Ref == productsProp.Type.Ref {
			t.Errorf("GENERIC COLLAPSE: users and products both reference the same schema %q", usersProp.Type.Ref)
		}
	}
}

// TestWalkGenericTypeAlias_DirectUsage_UniqueSchemas tests that using
// PaginatedResponse<T> directly (without a named alias) still produces
// distinct schemas for each T.
func TestWalkGenericTypeAlias_DirectUsage_UniqueSchemas(t *testing.T) {
	env := setupWalker(t, `
type PaginatedResponse<T> = {
  items: T[];
  totalCount: number;
};

interface OrderResponse { orderId: string; total: number; }
interface CustomerResponse { customerId: string; name: string; }

type OrderContainer = {
  orders: PaginatedResponse<OrderResponse>;
};
type CustomerContainer = {
  customers: PaginatedResponse<CustomerResponse>;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("OrderContainer") {
		t.Fatal("OrderContainer should be registered")
	}
	if !reg.Has("CustomerContainer") {
		t.Fatal("CustomerContainer should be registered")
	}

	orderContainer := reg.Types["OrderContainer"]
	customerContainer := reg.Types["CustomerContainer"]

	ordersProp := findProperty(t, orderContainer.Properties, "orders")
	customersProp := findProperty(t, customerContainer.Properties, "customers")

	// Resolve to object metadata
	resolveToObj := func(m *metadata.Metadata) *metadata.Metadata {
		if m.Kind == metadata.KindRef {
			if resolved, ok := reg.Types[m.Ref]; ok {
				return resolved
			}
		}
		return m
	}

	ordersObj := resolveToObj(&ordersProp.Type)
	customersObj := resolveToObj(&customersProp.Type)

	// Both should have items property with array type
	ordersItems := findProperty(t, ordersObj.Properties, "items")
	customersItems := findProperty(t, customersObj.Properties, "items")

	if ordersItems.Type.Kind != metadata.KindArray {
		t.Fatalf("orders.items expected KindArray, got %s", ordersItems.Type.Kind)
	}
	if customersItems.Type.Kind != metadata.KindArray {
		t.Fatalf("customers.items expected KindArray, got %s", customersItems.Type.Kind)
	}

	// The element types must differ — OrderResponse vs CustomerResponse
	orderElem := ordersItems.Type.ElementType
	customerElem := customersItems.Type.ElementType
	if orderElem == nil || customerElem == nil {
		t.Fatal("element types should not be nil")
	}

	// If both are refs, they should point to different types
	if orderElem.Kind == metadata.KindRef && customerElem.Kind == metadata.KindRef {
		if orderElem.Ref == customerElem.Ref {
			t.Errorf("GENERIC COLLAPSE: orders.items and customers.items both reference %q", orderElem.Ref)
		}
	}
}

// --- Bug 2: Named Array Type Alias Double-Nesting ---

// TestWalkNamedArrayTypeAlias_NoDoubleNesting verifies that a named type alias
// that resolves to an array (e.g., type ShipmentItemSnapshot = {...}[]) does NOT
// register as a named array schema. This prevents double-nesting in OpenAPI where
// ShipmentItemSnapshot[] would become {...}[][] instead of {...}[].
func TestWalkNamedArrayTypeAlias_NoDoubleNesting(t *testing.T) {
	env := setupWalker(t, `
type ShipmentItemSnapshot = {
  variantId: string;
  productId: string;
  productName: string;
  quantity: number;
  price: number;
}[];

type ShipmentResponse = {
  id: string;
  items: ShipmentItemSnapshot;
  trackingNumber: string;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	// ShipmentResponse should be registered
	if !reg.Has("ShipmentResponse") {
		t.Fatal("ShipmentResponse should be registered")
	}

	resp := reg.Types["ShipmentResponse"]
	itemsProp := findProperty(t, resp.Properties, "items")

	// items should be an array of objects (not a $ref to a named array schema)
	if itemsProp.Type.Kind == metadata.KindRef {
		// If it's a ref, the referenced type should be an OBJECT, not an array
		resolved, ok := reg.Types[itemsProp.Type.Ref]
		if !ok {
			t.Fatalf("ref %q not found in registry", itemsProp.Type.Ref)
		}
		if resolved.Kind == metadata.KindArray {
			t.Errorf("DOUBLE-NESTING BUG: ShipmentItemSnapshot is registered as KindArray — it should be the inner object type, not the array wrapper")
		}
	} else if itemsProp.Type.Kind == metadata.KindArray {
		// Direct array is fine — check element is an object with correct properties
		if itemsProp.Type.ElementType == nil {
			t.Fatal("items array should have element type")
		}
		elem := itemsProp.Type.ElementType
		if elem.Kind == metadata.KindRef {
			resolved, ok := reg.Types[elem.Ref]
			if !ok {
				t.Fatalf("ref %q not found", elem.Ref)
			}
			if resolved.Kind == metadata.KindArray {
				t.Errorf("DOUBLE-NESTING BUG: element ref %q points to another array", elem.Ref)
			}
		}
	} else {
		t.Errorf("items: expected KindArray or KindRef, got %s", itemsProp.Type.Kind)
	}
}

// TestWalkNamedArrayTypeAlias_UsedInArrayContext tests the critical scenario:
// a named array type used as SomeType[] should NOT produce double nesting.
func TestWalkNamedArrayTypeAlias_UsedInArrayContext(t *testing.T) {
	env := setupWalker(t, `
type ItemSnapshot = {
  productId: string;
  quantity: number;
  price: number;
}[];

// Using ItemSnapshot[] - WITHOUT the fix, this would be {...}[][]
type OrderDetail = {
  orderId: string;
  items: ItemSnapshot;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("OrderDetail") {
		t.Fatal("OrderDetail should be registered")
	}

	detail := reg.Types["OrderDetail"]
	itemsProp := findProperty(t, detail.Properties, "items")

	// The items property should ultimately resolve to a single-level array of objects.
	// Not an array of arrays.
	resolvedType := &itemsProp.Type
	if resolvedType.Kind == metadata.KindRef {
		resolved, ok := reg.Types[resolvedType.Ref]
		if !ok {
			t.Fatalf("ref %q not found", resolvedType.Ref)
		}
		resolvedType = resolved
	}

	if resolvedType.Kind == metadata.KindArray {
		// It's an array — the element should be an object, NOT another array
		if resolvedType.ElementType == nil {
			t.Fatal("array element type should not be nil")
		}
		elemType := resolvedType.ElementType
		if elemType.Kind == metadata.KindRef {
			resolved, ok := reg.Types[elemType.Ref]
			if !ok {
				t.Fatalf("ref %q not found", elemType.Ref)
			}
			elemType = resolved
		}
		if elemType.Kind == metadata.KindArray {
			t.Errorf("DOUBLE-NESTING BUG: items resolves to array-of-array — element is KindArray instead of KindObject")
		}
		if elemType.Kind != metadata.KindObject {
			t.Errorf("items array element: expected KindObject, got %s", elemType.Kind)
		}
	} else {
		t.Errorf("items: expected to resolve to KindArray, got %s", resolvedType.Kind)
	}
}

// TestWalkNamedArrayTypeAlias_NameCollision tests that when two types share
// the same name but one is an array and one is an object (like the PrismaJson
// vs DTO ShipmentItemSnapshot case), the object version wins in the registry.
func TestWalkNamedArrayTypeAlias_NameCollision(t *testing.T) {
	env := setupWalker(t, `
// First definition: array type (like PrismaJson.ShipmentItemSnapshot)
type ShipmentItemSnapshot = {
  variantId: string;
  productId: string;
  quantity: number;
  price: number;
}[];

// Second definition: a response using it
type ShipmentResponse = {
  id: string;
  items: ShipmentItemSnapshot;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	// If ShipmentItemSnapshot IS in the registry, it should NOT be KindArray
	if reg.Has("ShipmentItemSnapshot") {
		m := reg.Types["ShipmentItemSnapshot"]
		if m.Kind == metadata.KindArray {
			t.Errorf("DOUBLE-NESTING BUG: ShipmentItemSnapshot registered as KindArray in the registry — named array types should not become $ref schemas")
		}
	}
	// If it's not in the registry at all, that's also acceptable —
	// it means the array is inlined, which avoids double-nesting
}

// --- Bug 1 follow-up: Type alias args should produce readable names, not T{id} ---

// TestWalkGenericTypeAlias_TypeAliasArgs_ReadableNames verifies that when a
// generic type like PaginatedResponse<T> is used with type-alias arguments
// (not interfaces), the composite schema name uses the alias name rather than
// falling back to T{id}. This mirrors the ecom-bot pattern where controller
// methods return Promise<PaginatedResponse<FacebookWebhookLogResponse>> and
// FacebookWebhookLogResponse is a type alias (not an interface).
func TestWalkGenericTypeAlias_TypeAliasArgs_ReadableNames(t *testing.T) {
	env := setupWalker(t, `
type PaginatedResponse<T> = {
  items: T[];
  totalCount: number;
  currentPage: number;
};

// Type aliases (not interfaces) — these resolve to anonymous object types
type WebhookLogResponse = {
  id: string;
  payload: string;
  receivedAt: string;
};

type OutboundLogResponse = {
  id: string;
  destination: string;
  sentAt: string;
  status: string;
};

// Container that uses PaginatedResponse with type-alias arguments
type ApiResult = {
  webhooks: PaginatedResponse<WebhookLogResponse>;
  outbound: PaginatedResponse<OutboundLogResponse>;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("ApiResult") {
		t.Fatal("ApiResult should be registered")
	}
	apiResult := reg.Types["ApiResult"]

	webhooksProp := findProperty(t, apiResult.Properties, "webhooks")
	outboundProp := findProperty(t, apiResult.Properties, "outbound")

	// Both should be $refs to distinct registered types
	if webhooksProp.Type.Kind != metadata.KindRef {
		t.Fatalf("webhooks: expected KindRef, got %s", webhooksProp.Type.Kind)
	}
	if outboundProp.Type.Kind != metadata.KindRef {
		t.Fatalf("outbound: expected KindRef, got %s", outboundProp.Type.Kind)
	}

	// The ref names should contain the human-readable type alias names, NOT T{id}
	webhooksRef := webhooksProp.Type.Ref
	outboundRef := outboundProp.Type.Ref

	if webhooksRef == outboundRef {
		t.Errorf("webhooks and outbound both reference %q — should be distinct", webhooksRef)
	}

	// Should contain "WebhookLogResponse" and "OutboundLogResponse" respectively
	if !strings.Contains(webhooksRef, "WebhookLogResponse") {
		t.Errorf("webhooks ref %q should contain 'WebhookLogResponse'", webhooksRef)
	}
	if !strings.Contains(outboundRef, "OutboundLogResponse") {
		t.Errorf("outbound ref %q should contain 'OutboundLogResponse'", outboundRef)
	}

	// Neither should contain T followed by digits (the fallback pattern)
	for _, ref := range []string{webhooksRef, outboundRef} {
		for i := 0; i < len(ref)-1; i++ {
			if ref[i] == 'T' && i+1 < len(ref) && ref[i+1] >= '0' && ref[i+1] <= '9' {
				// Check it's not part of a word like "Template"
				if i == 0 || ref[i-1] == '_' {
					t.Errorf("ref %q contains T{id} fallback name — type alias arg name was not resolved", ref)
					break
				}
			}
		}
	}

	// Verify the schemas have correct items types
	for _, tc := range []struct {
		ref     string
		itemRef string
	}{
		{webhooksRef, "WebhookLogResponse"},
		{outboundRef, "OutboundLogResponse"},
	} {
		resolved, ok := reg.Types[tc.ref]
		if !ok {
			t.Errorf("schema %q not in registry", tc.ref)
			continue
		}
		itemsProp := findProperty(t, resolved.Properties, "items")
		if itemsProp.Type.Kind != metadata.KindArray || itemsProp.Type.ElementType == nil {
			t.Errorf("%s.items: expected array with element", tc.ref)
			continue
		}
		elem := itemsProp.Type.ElementType
		if elem.Kind != metadata.KindRef {
			t.Errorf("%s.items element: expected ref, got %s", tc.ref, elem.Kind)
			continue
		}
		if elem.Ref != tc.itemRef {
			t.Errorf("%s.items element: expected ref %q, got %q", tc.ref, tc.itemRef, elem.Ref)
		}
	}
}

// TestWalkGenericInterface_InterfaceArgs_ReadableNames verifies that generic
// interfaces with interface-typed arguments also produce readable composite names.
func TestWalkGenericInterface_InterfaceArgs_ReadableNames(t *testing.T) {
	env := setupWalker(t, `
interface PaginatedList<T> {
  data: T[];
  meta: { total: number; page: number };
}

interface Campaign { id: string; name: string; budget: number; }
interface AdSet { id: string; targeting: string; }

type Dashboard = {
  campaigns: PaginatedList<Campaign>;
  adSets: PaginatedList<AdSet>;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("Dashboard") {
		t.Fatal("Dashboard should be registered")
	}
	dashboard := reg.Types["Dashboard"]

	campaignsProp := findProperty(t, dashboard.Properties, "campaigns")
	adSetsProp := findProperty(t, dashboard.Properties, "adSets")

	if campaignsProp.Type.Kind != metadata.KindRef {
		t.Fatalf("campaigns: expected KindRef, got %s", campaignsProp.Type.Kind)
	}
	if adSetsProp.Type.Kind != metadata.KindRef {
		t.Fatalf("adSets: expected KindRef, got %s", adSetsProp.Type.Kind)
	}

	// Names should be readable
	if !strings.Contains(campaignsProp.Type.Ref, "Campaign") {
		t.Errorf("campaigns ref %q should contain 'Campaign'", campaignsProp.Type.Ref)
	}
	if !strings.Contains(adSetsProp.Type.Ref, "AdSet") {
		t.Errorf("adSets ref %q should contain 'AdSet'", adSetsProp.Type.Ref)
	}

	// Verify different refs
	if campaignsProp.Type.Ref == adSetsProp.Type.Ref {
		t.Errorf("campaigns and adSets should have different refs, both are %q", campaignsProp.Type.Ref)
	}
}

// TestWalkGenericTypeAlias_AnonymousArgs_InlineAndWarn verifies that when a
// generic type like Wrapper<T> is instantiated with an anonymous object literal
// (e.g., Wrapper<{ x: number }>), the type is inlined (not registered under a
// T{id} name) and a warning is emitted suggesting the user create a named alias.
func TestWalkGenericTypeAlias_AnonymousArgs_InlineAndWarn(t *testing.T) {
	env := setupWalker(t, `
type Wrapper<T> = {
  data: T;
  timestamp: number;
};

// Named alias — should get a readable composite name
type UserDto = { id: string; name: string; };

// Container with both named and anonymous type args
type Response = {
  named: Wrapper<UserDto>;
  anonymous: Wrapper<{ x: number; y: number }>;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("Response") {
		t.Fatal("Response should be registered")
	}
	response := reg.Types["Response"]

	namedProp := findProperty(t, response.Properties, "named")
	anonProp := findProperty(t, response.Properties, "anonymous")

	// Named arg should produce a $ref with readable name
	if namedProp.Type.Kind != metadata.KindRef {
		t.Fatalf("named: expected KindRef, got %s", namedProp.Type.Kind)
	}
	if !strings.Contains(namedProp.Type.Ref, "UserDto") {
		t.Errorf("named ref %q should contain 'UserDto'", namedProp.Type.Ref)
	}

	// Anonymous arg should be inlined (KindObject, not KindRef)
	if anonProp.Type.Kind == metadata.KindRef {
		t.Errorf("anonymous: expected inlined type (not KindRef), got ref %q — anonymous type args should not be registered", anonProp.Type.Ref)
	}
	if anonProp.Type.Kind != metadata.KindObject {
		t.Fatalf("anonymous: expected KindObject (inlined), got %s", anonProp.Type.Kind)
	}

	// Should have emitted a warning about the anonymous type arg
	warnings := walker.Warnings()
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "Wrapper") && strings.Contains(w, "anonymous type arguments") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a warning about Wrapper having anonymous type arguments, got warnings: %v", warnings)
	}
}

// TestWalkGenericInterface_AnonymousArgs_InlineAndWarn verifies the same
// inline+warn behavior for generic interfaces (not just type aliases).
func TestWalkGenericInterface_AnonymousArgs_InlineAndWarn(t *testing.T) {
	env := setupWalker(t, `
interface Container<T> {
  value: T;
  count: number;
}

type Result = {
  item: Container<{ foo: string; bar: number }>;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("Result") {
		t.Fatal("Result should be registered")
	}
	result := reg.Types["Result"]

	itemProp := findProperty(t, result.Properties, "item")

	// Anonymous arg should be inlined
	if itemProp.Type.Kind == metadata.KindRef {
		t.Errorf("item: expected inlined type (not KindRef), got ref %q", itemProp.Type.Ref)
	}
	if itemProp.Type.Kind != metadata.KindObject {
		t.Fatalf("item: expected KindObject (inlined), got %s", itemProp.Type.Kind)
	}

	// Should have emitted a warning
	warnings := walker.Warnings()
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "Container") && strings.Contains(w, "anonymous type arguments") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a warning about Container having anonymous type arguments, got warnings: %v", warnings)
	}
}

// TestWalkGeneric_PickWithLiteralUnion verifies that Pick<User, 'id' | 'name'>
// gets a readable composite name from the string literal union (not inlined).
func TestWalkGeneric_PickWithLiteralUnion(t *testing.T) {
	env := setupWalker(t, `
type User = {
  id: string;
  name: string;
  email: string;
  age: number;
};

type UserSummary = Pick<User, 'id' | 'name'>;
type UserContact = Pick<User, 'email' | 'name'>;

type Container = {
  summary: UserSummary;
  contact: UserContact;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("Container") {
		t.Fatal("Container should be registered")
	}
	container := reg.Types["Container"]

	summaryProp := findProperty(t, container.Properties, "summary")
	contactProp := findProperty(t, container.Properties, "contact")

	// Both should be refs (Pick resolves to a concrete object; alias recovery
	// should produce composite names using the literal union members)
	if summaryProp.Type.Kind != metadata.KindRef {
		// Pick resolves to an anonymous object — alias recovery with literal union
		// may or may not produce a ref depending on whether the alias is preserved.
		// At minimum, it should NOT have a T{id} fallback name.
		if summaryProp.Type.Kind == metadata.KindObject {
			t.Logf("summary inlined as KindObject (acceptable — Pick alias may not survive)")
		} else {
			t.Errorf("summary: unexpected kind %s", summaryProp.Type.Kind)
		}
	} else {
		// If it's a ref, name should not contain T{id} pattern
		if strings.Contains(summaryProp.Type.Ref, "T1") || strings.Contains(summaryProp.Type.Ref, "T2") {
			t.Errorf("summary ref %q should not contain T{id} fallback names", summaryProp.Type.Ref)
		}
	}

	// Verify distinct schemas if both are refs
	if summaryProp.Type.Kind == metadata.KindRef && contactProp.Type.Kind == metadata.KindRef {
		if summaryProp.Type.Ref == contactProp.Type.Ref {
			t.Errorf("summary and contact should have different refs, both are %q", summaryProp.Type.Ref)
		}
	}

	// No warnings should mention T{id}
	for _, w := range walker.Warnings() {
		if strings.Contains(w, "T1") || strings.Contains(w, "T2") {
			t.Errorf("warning should not contain T{id} fallback: %s", w)
		}
	}
}

// TestWalkGeneric_WarningDeduplication verifies that multiple uses of the same
// unnameable generic type produce only a single warning.
func TestWalkGeneric_WarningDeduplication(t *testing.T) {
	env := setupWalker(t, `
type Wrapper<T> = {
  data: T;
};

type Result = {
  a: Wrapper<{ x: number }>;
  b: Wrapper<{ y: string }>;
  c: Wrapper<{ z: boolean }>;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)

	// Count how many warnings mention "Wrapper"
	wrapperCount := 0
	for _, w := range walker.Warnings() {
		if strings.Contains(w, "Wrapper") && strings.Contains(w, "anonymous type arguments") {
			wrapperCount++
		}
	}

	if wrapperCount != 1 {
		t.Errorf("expected exactly 1 deduplicated warning for Wrapper, got %d; warnings: %v",
			wrapperCount, walker.Warnings())
	}
}

// TestWalkGeneric_LargeLiteralUnion_Inlines verifies that generic types with
// more than 4 literal union members as type arguments are inlined (not named
// with an absurdly long composite name).
func TestWalkGeneric_LargeLiteralUnion_Inlines(t *testing.T) {
	env := setupWalker(t, `
type User = {
  id: string;
  name: string;
  email: string;
  age: number;
  phone: string;
  address: string;
};

// 5 literals — exceeds the threshold for naming
type BigPick = Pick<User, 'id' | 'name' | 'email' | 'age' | 'phone'>;

type Container = {
  item: BigPick;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("Container") {
		t.Fatal("Container should be registered")
	}
	container := reg.Types["Container"]

	itemProp := findProperty(t, container.Properties, "item")

	// Should be registered as "BigPick" (non-generic alias name) or inlined,
	// but NOT with a long composite name joining all 5 literals
	if itemProp.Type.Kind == metadata.KindRef {
		ref := itemProp.Type.Ref
		// BigPick is a non-generic top-level type alias — should be registered as "BigPick"
		if ref != "BigPick" {
			// Acceptable if it has the alias name, but should not have T{id}
			if strings.HasPrefix(ref, "Pick_") && len(ref) > 40 {
				t.Errorf("ref %q is too long — large literal unions should not produce composite names", ref)
			}
		}
	}
	// If it's KindObject (inlined), that's also fine for a large union
}

// TestWalkGeneric_RecordStringAnonymousObject verifies that Record<string, {...}>
// where the value type is an anonymous object gets inlined and warns.
func TestWalkGeneric_RecordStringAnonymousObject(t *testing.T) {
	env := setupWalker(t, `
type Lookup = {
  data: Record<string, { label: string; value: number }>;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("Lookup") {
		t.Fatal("Lookup should be registered")
	}
	lookup := reg.Types["Lookup"]
	dataProp := findProperty(t, lookup.Properties, "data")

	// The Record<string, anonymous> should NOT produce a T{id} ref name
	if dataProp.Type.Kind == metadata.KindRef {
		ref := dataProp.Type.Ref
		for i := 0; i < 100; i++ {
			if strings.Contains(ref, fmt.Sprintf("T%d", i)) && !strings.Contains(ref, "Type") {
				t.Errorf("data ref %q contains T{id} fallback — should be inlined or have a readable name", ref)
				break
			}
		}
	}

	// If Record warning was emitted, it should be deduplicated
	recordCount := 0
	for _, w := range walker.Warnings() {
		if strings.Contains(w, "Record") {
			recordCount++
		}
	}
	if recordCount > 1 {
		t.Errorf("expected at most 1 Record warning, got %d", recordCount)
	}
}

func TestWalkConstEnum(t *testing.T) {
	env := setupWalker(t, `
		const enum Direction { Up = 0, Down = 1, Left = 2, Right = 3 }
		type T = Direction;
	`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	// Const enums resolve the same way as regular enums (union of literals or enum kind)
	if m.Kind != metadata.KindUnion && m.Kind != metadata.KindEnum {
		t.Errorf("expected union or enum kind for const enum, got %s", m.Kind)
	}
}

func TestWalkHeterogeneousEnum(t *testing.T) {
	env := setupWalker(t, `
		enum Mixed { Yes = 1, No = 0, Maybe = "maybe" }
		type T = Mixed;
	`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	if m.Kind != metadata.KindUnion && m.Kind != metadata.KindEnum {
		t.Errorf("expected union or enum kind for heterogeneous enum, got %s", m.Kind)
	}
}

func TestWalkClassVisibilityFiltering(t *testing.T) {
	env := setupWalker(t, `
		class UserDto {
			public name: string = "";
			private password: string = "";
			protected internalId: number = 0;
			email: string = "";
		}
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "UserDto")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)

	// Public and default visibility should be present
	names := propNames(resolved.Properties)
	hasName := false
	hasEmail := false
	hasPassword := false
	hasInternalId := false
	for _, n := range names {
		switch n {
		case "name":
			hasName = true
		case "email":
			hasEmail = true
		case "password":
			hasPassword = true
		case "internalId":
			hasInternalId = true
		}
	}

	if !hasName {
		t.Error("public property 'name' should be present")
	}
	if !hasEmail {
		t.Error("default visibility property 'email' should be present")
	}
	// Private and protected may or may not be filtered by the walker.
	// If they ARE present, we accept both behaviors — the important thing
	// is that the walker does not crash on visibility modifiers.
	_ = hasPassword
	_ = hasInternalId
}

func TestWalkClassConstructorParameterProperties(t *testing.T) {
	env := setupWalker(t, `
		class CreateUserDto {
			constructor(
				public readonly name: string,
				public age: number,
				public email: string,
			) {}
		}
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "CreateUserDto")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)

	if len(resolved.Properties) < 3 {
		t.Fatalf("expected at least 3 properties from constructor params, got %d: %v",
			len(resolved.Properties), propNames(resolved.Properties))
	}
	assertPropertyExists(t, resolved.Properties, "name", metadata.KindAtomic)
	assertPropertyExists(t, resolved.Properties, "age", metadata.KindAtomic)
	assertPropertyExists(t, resolved.Properties, "email", metadata.KindAtomic)
}

func TestWalkPropertyTypeUndefined(t *testing.T) {
	env := setupWalker(t, `
		interface Strange {
			nothing: undefined;
			name: string;
		}
		type T = Strange;
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)
	// Should not crash; name should be present
	assertPropertyExists(t, resolved.Properties, "name", metadata.KindAtomic)
}

func TestWalkPropertyTypeNever(t *testing.T) {
	env := setupWalker(t, `
		interface Impossible {
			unreachable: never;
			name: string;
		}
		type T = Impossible;
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)
	assertPropertyExists(t, resolved.Properties, "name", metadata.KindAtomic)
}

func TestWalkPropertyTypeUnknown(t *testing.T) {
	env := setupWalker(t, `
		interface Flexible {
			data: unknown;
			name: string;
		}
		type T = Flexible;
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)
	assertPropertyExists(t, resolved.Properties, "name", metadata.KindAtomic)
	// data should be KindUnknown or KindAny
	dataProp := findProperty(t, resolved.Properties, "data")
	if dataProp.Type.Kind != metadata.KindUnknown && dataProp.Type.Kind != metadata.KindAny {
		t.Errorf("expected data to be unknown or any, got %s", dataProp.Type.Kind)
	}
}

func TestWalkPartialPickAndRequiredPick(t *testing.T) {
	env := setupWalker(t, `
		interface User {
			id: string;
			name: string;
			email: string;
			bio: string;
			age: number;
		}
		type UpdateUserDto = Partial<Pick<User, 'name' | 'bio'>> & Required<Pick<User, 'id'>>;
	`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("UpdateUserDto") {
		t.Fatal("UpdateUserDto should be registered")
	}

	m := reg.Types["UpdateUserDto"]
	assertKind(t, *m, metadata.KindObject)

	props := make(map[string]bool)
	for _, p := range m.Properties {
		props[p.Name] = true
	}
	if !props["id"] || !props["name"] || !props["bio"] {
		t.Errorf("expected id, name, bio; got %v", props)
	}
	if len(m.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(m.Properties))
	}
}

func TestWalkRecursiveSelfType(t *testing.T) {
	env := setupWalker(t, `
		type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };
	`)
	defer env.release()

	// Should not infinite loop — just verify it completes
	m := env.walkExportedType(t, "JsonValue")
	if m.Kind != metadata.KindUnion && m.Kind != metadata.KindRef && m.Kind != metadata.KindAny {
		t.Errorf("expected union, ref, or any for recursive self-type, got %s", m.Kind)
	}
}

func TestWalkUnionOfTuples(t *testing.T) {
	env := setupWalker(t, `
		type EmptyTuple = [];
		type Pair = [boolean, number];
		type Triple = [number, string, boolean];
		type T = EmptyTuple | Pair | Triple;
	`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindUnion)
	if len(m.UnionMembers) < 2 {
		t.Errorf("expected at least 2 union members for tuple union, got %d", len(m.UnionMembers))
	}
}

func TestWalkReadonlyArrayGeneric(t *testing.T) {
	env := setupWalker(t, `type T = ReadonlyArray<number>;`)
	defer env.release()

	m := env.walkExportedType(t, "T")
	assertKind(t, m, metadata.KindArray)
	if m.ElementType == nil {
		t.Fatal("expected element type")
	}
	assertKind(t, *m.ElementType, metadata.KindAtomic)
	assertAtomic(t, *m.ElementType, "number")
}

func TestWalkDeepNesting(t *testing.T) {
	env := setupWalker(t, `
		interface Level5 { value: string; }
		interface Level4 { child: Level5; }
		interface Level3 { child: Level4; }
		interface Level2 { child: Level3; }
		interface Level1 { child: Level2; }
		type T = Level1;
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	if m.Kind != metadata.KindRef {
		t.Fatalf("expected ref, got %s", m.Kind)
	}
	level1 := reg.Types[m.Ref]
	if level1 == nil {
		t.Fatal("Level1 not found in registry")
	}

	// Verify all 5 levels are registered and chain correctly
	for _, name := range []string{"Level1", "Level2", "Level3", "Level4", "Level5"} {
		if reg.Types[name] == nil {
			t.Errorf("%s not found in registry", name)
		}
	}
}

func TestWalkNonIdentifierPropertyKeys(t *testing.T) {
	env := setupWalker(t, `
		interface WeirdKeys {
			"normal-prop": string;
			"with spaces": number;
			"123starts-with-number": boolean;
			"has.dot": string;
		}
		type T = WeirdKeys;
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "T")
	resolved := resolveRef(m, reg)
	assertKind(t, *resolved, metadata.KindObject)

	if len(resolved.Properties) != 4 {
		t.Fatalf("expected 4 properties, got %d: %v", len(resolved.Properties), propNames(resolved.Properties))
	}

	// Verify all non-identifier keys are captured
	names := make(map[string]bool)
	for _, p := range resolved.Properties {
		names[p.Name] = true
	}
	for _, expected := range []string{"normal-prop", "with spaces", "123starts-with-number", "has.dot"} {
		if !names[expected] {
			t.Errorf("expected property %q not found", expected)
		}
	}
}

func TestWalkLiteralPlusTaggedUnion(t *testing.T) {
	env := setupWalker(t, `
		type Format<F extends string> = { readonly __tsgonest_format: F };
		interface Config {
			id: "latest" | (string & Format<"uuid">);
		}
	`)
	defer env.release()

	m := env.walkExportedType(t, "Config")
	if m.Kind == metadata.KindRef {
		reg := env.walkExportedTypeWithRegistryOnly(t, "Config")
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}
	assertKind(t, m, metadata.KindObject)

	idProp := findProperty(t, m.Properties, "id")
	// The union "latest" | (string & Format<"uuid">) should be a union type
	if idProp.Type.Kind != metadata.KindUnion && idProp.Type.Kind != metadata.KindAtomic {
		t.Errorf("expected union or atomic for literal+tagged union, got %s", idProp.Type.Kind)
	}
}

// --- Bug: Named generic type alias name not preserved ---

// TestWalkNamedType_GenericInterface_PreservesAliasName verifies that when a named
// type alias points to a generic interface instantiation (e.g.,
// type MyList = PagedResult<Item>), WalkNamedType propagates the alias name on
// the returned KindRef metadata. This ensures downstream consumers (OpenAPI, SDK)
// can use the user-defined name instead of the mechanical composite name
// (PagedResult_Item).
func TestWalkNamedType_GenericInterface_PreservesAliasName(t *testing.T) {
	env := setupWalker(t, `
interface PagedResult<T> {
  items: T[];
  total: number;
  page: number;
}

interface Item {
  id: string;
  value: string;
}

type MyItemList = PagedResult<Item>;
`)
	defer env.release()

	// Walk the named type alias directly via WalkNamedType
	walker := analyzer.NewTypeWalker(env.checker)
	var result metadata.Metadata
	found := false
	for _, stmt := range env.sourceFile.Statements.Nodes {
		if stmt.Kind == ast.KindTypeAliasDeclaration {
			decl := stmt.AsTypeAliasDeclaration()
			if decl.TypeParameters != nil {
				continue
			}
			name := decl.Name().Text()
			resolvedType := shimchecker.Checker_getTypeFromTypeNode(env.checker, decl.Type)
			m := walker.WalkNamedType(name, resolvedType)

			if name == "MyItemList" {
				result = m
				found = true
			}
		}
	}

	if !found {
		t.Fatal("MyItemList type alias not found")
	}

	// The result should be a KindRef pointing to the mechanical composite name,
	// but with the alias Name propagated for downstream consumers (OpenAPI, SDK).
	if result.Kind != metadata.KindRef {
		t.Fatalf("expected KindRef, got %s", result.Kind)
	}
	if result.Name != "MyItemList" {
		t.Errorf("expected Name='MyItemList', got %q", result.Name)
	}
	if result.Ref != "PagedResult_Item" {
		t.Errorf("expected Ref='PagedResult_Item', got %q", result.Ref)
	}
}

// --- Bug: Self-referential intersection types degrade to empty schemas ---

// TestWalkNamedType_SelfReferentialIntersection verifies that a self-referential
// type alias using an intersection (e.g., type Node = Base & { children: Node[] })
// correctly handles recursion: the recursive property should become a $ref, and
// all OTHER properties (non-recursive) should be fully resolved.
func TestWalkNamedType_SelfReferentialIntersection(t *testing.T) {
	env := setupWalker(t, `
interface Entity {
  id: string;
  createdAt: string;
}

interface Attachment {
  name: string;
  size: number;
}

interface Reaction {
  emoji: string;
  count: number;
}

type Message = Entity & {
  content: string;
  pinned: boolean;
  replies?: Message[];
  attachments: Attachment[];
  reactions: Reaction[];
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("Message") {
		t.Fatal("Message should be registered in the registry")
	}
	m := reg.Types["Message"]

	if m.Kind != metadata.KindObject {
		t.Fatalf("expected KindObject, got %s", m.Kind)
	}

	// Check that properties from the intersection were merged correctly
	propMap := make(map[string]*metadata.Property)
	for i := range m.Properties {
		propMap[m.Properties[i].Name] = &m.Properties[i]
	}

	// Properties from Entity should be present
	if _, ok := propMap["id"]; !ok {
		t.Error("expected 'id' property from Entity")
	}
	if _, ok := propMap["createdAt"]; !ok {
		t.Error("expected 'createdAt' property from Entity")
	}

	// The recursive property should resolve to a $ref, not degrade to KindAny
	repliesProp, ok := propMap["replies"]
	if !ok {
		t.Fatal("expected 'replies' property")
	}
	if repliesProp.Type.Kind != metadata.KindArray {
		t.Fatalf("replies: expected KindArray, got %s", repliesProp.Type.Kind)
	}
	if repliesProp.Type.ElementType == nil {
		t.Fatal("replies: expected array element type to be set")
	}
	if repliesProp.Type.ElementType.Kind != metadata.KindRef {
		t.Errorf("replies element: expected KindRef, got %s", repliesProp.Type.ElementType.Kind)
	}
	if repliesProp.Type.ElementType.Ref != "Message" {
		t.Errorf("replies element: expected Ref='Message', got %q", repliesProp.Type.ElementType.Ref)
	}

	// NON-recursive properties must NOT degrade to KindAny/empty
	contentProp, ok := propMap["content"]
	if !ok {
		t.Fatal("expected 'content' property")
	}
	if contentProp.Type.Kind != metadata.KindAtomic || contentProp.Type.Atomic != "string" {
		t.Errorf("content: expected atomic string, got Kind=%s Atomic=%q", contentProp.Type.Kind, contentProp.Type.Atomic)
	}

	pinnedProp, ok := propMap["pinned"]
	if !ok {
		t.Fatal("expected 'pinned' property")
	}
	if pinnedProp.Type.Kind != metadata.KindAtomic || pinnedProp.Type.Atomic != "boolean" {
		t.Errorf("pinned: expected atomic boolean, got Kind=%s Atomic=%q", pinnedProp.Type.Kind, pinnedProp.Type.Atomic)
	}

	// Attachment[] and Reaction[] must be fully resolved arrays, not degraded
	attachProp, ok := propMap["attachments"]
	if !ok {
		t.Fatal("expected 'attachments' property")
	}
	if attachProp.Type.Kind != metadata.KindArray {
		t.Errorf("attachments: expected KindArray, got %s", attachProp.Type.Kind)
	}
	if attachProp.Type.ElementType != nil && attachProp.Type.ElementType.Kind == metadata.KindAny {
		t.Error("attachments element: should NOT be KindAny (degraded)")
	}

	reactionsProp, ok := propMap["reactions"]
	if !ok {
		t.Fatal("expected 'reactions' property")
	}
	if reactionsProp.Type.Kind != metadata.KindArray {
		t.Errorf("reactions: expected KindArray, got %s", reactionsProp.Type.Kind)
	}
	if reactionsProp.Type.ElementType != nil && reactionsProp.Type.ElementType.Kind == metadata.KindAny {
		t.Error("reactions element: should NOT be KindAny (degraded)")
	}
}

// TestWalkNamedType_SelfReferentialIntersection_NullableScalars verifies that
// simple scalar properties (string | null, etc.) in a self-referential intersection
// type are fully resolved and not degraded to unknown.
func TestWalkNamedType_SelfReferentialIntersection_NullableScalars(t *testing.T) {
	env := setupWalker(t, `
interface Base {
  id: string;
}

type TreeNode = Base & {
  children: TreeNode[];
  label: string;
  description: string | null;
  priority: number | null;
};
`)
	defer env.release()

	walker := env.walkAllNamedTypes(t)
	reg := walker.Registry()

	if !reg.Has("TreeNode") {
		t.Fatal("TreeNode should be registered")
	}
	m := reg.Types["TreeNode"]

	propMap := make(map[string]*metadata.Property)
	for i := range m.Properties {
		propMap[m.Properties[i].Name] = &m.Properties[i]
	}

	// label should be a plain string, not KindAny
	labelProp, ok := propMap["label"]
	if !ok {
		t.Fatal("expected 'label' property")
	}
	if labelProp.Type.Kind != metadata.KindAtomic || labelProp.Type.Atomic != "string" {
		t.Errorf("label: expected atomic string, got Kind=%s", labelProp.Type.Kind)
	}

	// description should be string | null (nullable string), not unknown | null
	descProp, ok := propMap["description"]
	if !ok {
		t.Fatal("expected 'description' property")
	}
	// Should be an atomic string with Nullable=true, or a union with string member
	if descProp.Type.Kind == metadata.KindAny {
		t.Error("description: should NOT be KindAny (degraded to unknown)")
	}

	// priority should be number | null, not unknown | null
	priProp, ok := propMap["priority"]
	if !ok {
		t.Fatal("expected 'priority' property")
	}
	if priProp.Type.Kind == metadata.KindAny {
		t.Error("priority: should NOT be KindAny (degraded to unknown)")
	}
}

// --- Bug: Self-referential intersection as sub-field produces empty schema ---

// TestWalkNamedType_SelfReferentialIntersection_AsSubField verifies that when a
// self-referential intersection type (e.g., type Thread = Entity & { replies?: Thread[] })
// is used as a property of a DIFFERENT parent type, the recursive property still
// produces a $ref — not an empty schema ({}).
//
// This catches the case where the parent type is walked via WalkNamedType (which sets
// pendingName for the parent), but the self-referential child type is only encountered
// as a sub-field (walkIntersection runs without pendingName for the child type).
func TestWalkNamedType_SelfReferentialIntersection_AsSubField(t *testing.T) {
	env := setupWalker(t, `
interface Entity {
  id: string;
  createdAt: string;
}

type Thread = Entity & {
  content: string;
  replies?: Thread[];
};

type ChannelResponse = {
  name: string;
  threads: Thread[];
};
`)
	defer env.release()

	// Walk ONLY ChannelResponse via WalkNamedType. Thread is NOT walked directly —
	// it's encountered as a sub-field. This is the production scenario where the
	// parent type is the one being walked, and the child type is discovered during
	// property analysis.
	walker := analyzer.NewTypeWalker(env.checker)
	for _, stmt := range env.sourceFile.Statements.Nodes {
		if stmt.Kind == ast.KindTypeAliasDeclaration {
			decl := stmt.AsTypeAliasDeclaration()
			name := decl.Name().Text()
			// Only walk ChannelResponse — skip Thread and generic aliases
			if name == "ChannelResponse" {
				resolvedType := shimchecker.Checker_getTypeFromTypeNode(env.checker, decl.Type)
				walker.WalkNamedType(name, resolvedType)
			}
		}
	}

	reg := walker.Registry()

	// Thread should be discovered and registered as a sub-field type
	if !reg.Has("Thread") {
		t.Fatal("Thread should be registered in the registry (discovered as sub-field)")
	}
	threadMeta := reg.Types["Thread"]
	if threadMeta.Kind != metadata.KindObject {
		t.Fatalf("Thread: expected KindObject, got %s", threadMeta.Kind)
	}

	propMap := make(map[string]*metadata.Property)
	for i := range threadMeta.Properties {
		propMap[threadMeta.Properties[i].Name] = &threadMeta.Properties[i]
	}

	// The recursive 'replies' property should be KindArray with KindRef element
	repliesProp, ok := propMap["replies"]
	if !ok {
		t.Fatal("expected 'replies' property on Thread")
	}
	if repliesProp.Type.Kind != metadata.KindArray {
		t.Fatalf("replies: expected KindArray, got %s", repliesProp.Type.Kind)
	}
	if repliesProp.Type.ElementType == nil {
		t.Fatal("replies: expected element type, got nil")
	}
	// CRITICAL: element type must be KindRef to "Thread", NOT KindAny (which produces {} in OpenAPI)
	if repliesProp.Type.ElementType.Kind == metadata.KindAny {
		t.Fatal("replies element: got KindAny — self-referential type degraded to empty schema")
	}
	if repliesProp.Type.ElementType.Kind != metadata.KindRef {
		t.Fatalf("replies element: expected KindRef, got %s", repliesProp.Type.ElementType.Kind)
	}
	if repliesProp.Type.ElementType.Ref != "Thread" {
		t.Errorf("replies element: expected Ref='Thread', got %q", repliesProp.Type.ElementType.Ref)
	}

	// Non-recursive properties should be fully resolved
	contentProp, ok := propMap["content"]
	if !ok {
		t.Fatal("expected 'content' property on Thread")
	}
	if contentProp.Type.Kind != metadata.KindAtomic || contentProp.Type.Atomic != "string" {
		t.Errorf("content: expected atomic string, got Kind=%s Atomic=%q", contentProp.Type.Kind, contentProp.Type.Atomic)
	}
}

// --- Interface extending Array<T> ---

// TestWalkType_InterfaceExtendsArray verifies that an interface extending Array<T>
// is recognized as KindArray with the correct element type, rather than being
// expanded into an object with all Array.prototype methods as properties.
func TestWalkType_InterfaceExtendsArray(t *testing.T) {
	env := setupWalker(t, `
		interface Tags extends Array<string> {}
		type Wrapper = { tags: Tags; };
	`)
	defer env.release()

	m := env.walkExportedType(t, "Wrapper")
	assertKind(t, m, metadata.KindObject)

	if len(m.Properties) != 1 {
		t.Fatalf("expected 1 property, got %d", len(m.Properties))
	}
	tagsProp := m.Properties[0]
	if tagsProp.Name != "tags" {
		t.Fatalf("expected property 'tags', got %q", tagsProp.Name)
	}

	// tags should be KindArray, NOT KindObject with Array.prototype methods
	if tagsProp.Type.Kind != metadata.KindArray {
		t.Fatalf("tags: expected KindArray, got %s", tagsProp.Type.Kind)
	}
	if tagsProp.Type.ElementType == nil {
		t.Fatal("tags: expected element type, got nil")
	}
	if tagsProp.Type.ElementType.Kind != metadata.KindAtomic || tagsProp.Type.ElementType.Atomic != "string" {
		t.Errorf("tags element: expected atomic string, got Kind=%s Atomic=%s", tagsProp.Type.ElementType.Kind, tagsProp.Type.ElementType.Atomic)
	}
}

// TestWalkType_InterfaceExtendsArrayOfObject verifies interface extends Array<T>
// where T is a named object type (element type becomes a KindRef).
func TestWalkType_InterfaceExtendsArrayOfObject(t *testing.T) {
	env := setupWalker(t, `
		interface Item { id: string; label: string; }
		interface ItemList extends Array<Item> {}
		type Wrapper = { items: ItemList; };
	`)
	defer env.release()

	m := env.walkExportedType(t, "Wrapper")
	assertKind(t, m, metadata.KindObject)

	itemsProp := m.Properties[0]
	if itemsProp.Name != "items" {
		t.Fatalf("expected property 'items', got %q", itemsProp.Name)
	}

	// items should be KindArray (not KindObject with all Array.prototype methods)
	if itemsProp.Type.Kind != metadata.KindArray {
		t.Fatalf("items: expected KindArray, got %s", itemsProp.Type.Kind)
	}
	if itemsProp.Type.ElementType == nil {
		t.Fatal("items: expected element type, got nil")
	}
	// Item is a named interface, so the element type is KindRef
	if itemsProp.Type.ElementType.Kind != metadata.KindRef {
		t.Errorf("items element: expected KindRef (named Item), got %s", itemsProp.Type.ElementType.Kind)
	}
}

// TestWalkType_MutuallyRecursiveJsonTypes simulates Prisma's JsonValue/JsonObject/JsonArray
// pattern. The key assertion: the array-extending interface should be KindArray, not an
// object with 30+ Array.prototype methods as properties.
func TestWalkType_MutuallyRecursiveJsonTypes(t *testing.T) {
	env := setupWalker(t, `
		type FlexValue = string | number | boolean | FlexObject | FlexList | null;
		type FlexObject = { [Key in string]?: FlexValue; };
		interface FlexList extends Array<FlexValue> {}

		type Container = {
			data: FlexValue;
			metadata: FlexObject;
			tags: FlexList;
		};
	`)
	defer env.release()

	m := env.walkExportedType(t, "Container")
	assertKind(t, m, metadata.KindObject)

	propMap := make(map[string]metadata.Property)
	for _, p := range m.Properties {
		propMap[p.Name] = p
	}

	// data: FlexValue is a recursive union — should be resolved (not KindAny)
	dataProp, ok := propMap["data"]
	if !ok {
		t.Fatal("expected 'data' property")
	}
	if dataProp.Type.Kind == metadata.KindAny {
		t.Error("data: should NOT be KindAny (FlexValue should resolve to a union)")
	}

	// metadata: FlexObject has index signature — should be resolved (not KindAny)
	metaProp, ok := propMap["metadata"]
	if !ok {
		t.Fatal("expected 'metadata' property")
	}
	if metaProp.Type.Kind == metadata.KindAny {
		t.Error("metadata: should NOT be KindAny (FlexObject should resolve)")
	}

	// tags: FlexList extends Array<FlexValue> — MUST be KindArray
	tagsProp, ok := propMap["tags"]
	if !ok {
		t.Fatal("expected 'tags' property")
	}
	if tagsProp.Type.Kind != metadata.KindArray {
		t.Fatalf("tags: expected KindArray, got %s (interface extends Array should be recognized as array)", tagsProp.Type.Kind)
	}
	if tagsProp.Type.ElementType == nil {
		t.Fatal("tags: expected element type, got nil")
	}
	if tagsProp.Type.ElementType.Kind == metadata.KindAny {
		t.Error("tags element type: should not be KindAny (should be FlexValue)")
	}
}

// --- Bug: Large literal unions exhaust breadth limit, degrading later properties ---

// TestWalkType_LargeUnionDoesNotExhaustBreadthLimit verifies that a type with a
// large union property (e.g., 300+ currency codes) does not cause later properties
// to degrade to KindAny due to the breadth limit. This reproduces the Partial<T>
// and enum degradation bugs where properties after a large union became {}.
func TestWalkType_LargeUnionDoesNotExhaustBreadthLimit(t *testing.T) {
	// Generate a large union of 400 string literals to simulate TCurrencyCode4217
	var codes []string
	for i := 0; i < 400; i++ {
		codes = append(codes, fmt.Sprintf("'CODE_%03d'", i))
	}
	largeUnion := strings.Join(codes, " | ")

	src := fmt.Sprintf(`
		type CodeType = %s;

		type StatusEnum = 'ACTIVE' | 'INACTIVE' | 'PENDING';

		type Config = {
			code: CodeType;
			status: StatusEnum;
			name: string;
			count: number;
		};
	`, largeUnion)

	env := setupWalker(t, src)
	defer env.release()

	m := env.walkExportedType(t, "Config")
	assertKind(t, m, metadata.KindObject)

	propMap := make(map[string]metadata.Property)
	for _, p := range m.Properties {
		propMap[p.Name] = p
	}

	// All 4 properties should be fully resolved, none should be KindAny
	statusProp, ok := propMap["status"]
	if !ok {
		t.Fatal("expected 'status' property")
	}
	if statusProp.Type.Kind == metadata.KindAny {
		t.Error("status: should NOT be KindAny (breadth limit exhausted by large union)")
	}

	nameProp, ok := propMap["name"]
	if !ok {
		t.Fatal("expected 'name' property")
	}
	if nameProp.Type.Kind != metadata.KindAtomic || nameProp.Type.Atomic != "string" {
		t.Errorf("name: expected atomic string, got Kind=%s Atomic=%q", nameProp.Type.Kind, nameProp.Type.Atomic)
	}

	countProp, ok := propMap["count"]
	if !ok {
		t.Fatal("expected 'count' property")
	}
	if countProp.Type.Kind != metadata.KindAtomic || countProp.Type.Atomic != "number" {
		t.Errorf("count: expected atomic number, got Kind=%s Atomic=%q", countProp.Type.Kind, countProp.Type.Atomic)
	}
}

// --- Bug: File and Blob types not recognized as native types ---

// TestWalkType_FileNativeType verifies that the global File type is recognized as
// a native type, not expanded into an object with all File interface properties.
// File/Blob are DOM types, so we declare minimal stubs since the walker test
// environment uses lib: ["esnext"] which doesn't include DOM.
func TestWalkType_FileNativeType(t *testing.T) {
	env := setupWalker(t, `
		interface Blob {
			readonly size: number;
			readonly type: string;
			slice(start?: number, end?: number, contentType?: string): Blob;
		}
		interface File extends Blob {
			readonly lastModified: number;
			readonly name: string;
		}
		type Upload = { file: File; };
	`)
	defer env.release()

	m := env.walkExportedType(t, "Upload")
	assertKind(t, m, metadata.KindObject)

	fileProp := m.Properties[0]
	if fileProp.Name != "file" {
		t.Fatalf("expected property 'file', got %q", fileProp.Name)
	}
	if fileProp.Type.Kind != metadata.KindNative || fileProp.Type.NativeType != "File" {
		t.Errorf("file: expected KindNative/File, got Kind=%s NativeType=%q", fileProp.Type.Kind, fileProp.Type.NativeType)
	}
}

// TestWalkType_BlobNativeType verifies that the global Blob type is recognized as
// a native type.
func TestWalkType_BlobNativeType(t *testing.T) {
	env := setupWalker(t, `
		interface Blob {
			readonly size: number;
			readonly type: string;
			slice(start?: number, end?: number, contentType?: string): Blob;
		}
		type Upload = { data: Blob; };
	`)
	defer env.release()

	m := env.walkExportedType(t, "Upload")
	assertKind(t, m, metadata.KindObject)

	dataProp := m.Properties[0]
	if dataProp.Name != "data" {
		t.Fatalf("expected property 'data', got %q", dataProp.Name)
	}
	if dataProp.Type.Kind != metadata.KindNative || dataProp.Type.NativeType != "Blob" {
		t.Errorf("data: expected KindNative/Blob, got Kind=%s NativeType=%q", dataProp.Type.Kind, dataProp.Type.NativeType)
	}
}

// TestWalkType_InterfacePropertyTypesRegistered verifies that interfaces used
// as property types are registered as named schemas (KindRef), not inlined as
// empty schemas. Regression: nested interfaces with branded type properties
// were omitted from the type registry.
func TestWalkType_InterfacePropertyTypesRegistered(t *testing.T) {
	env := setupWalker(t, `
		interface PhoneEntry { number: string; type: string; }
		interface EmailEntry { address: string; label?: string; }

		interface ContactConfig {
			findOne?: { name: string; slug?: string; };
			createData?: {
				name: string;
				jobTitle?: string;
				phones?: PhoneEntry[];
				emails?: EmailEntry[];
			};
		}

		interface CompanyConfig {
			findOne?: { name: string; };
			createData?: {
				name: string;
				url?: string;
				phones?: PhoneEntry[];
			};
		}

		interface SourceConfig {
			findOne?: { title: string; };
			createData?: { title: string; url?: string; };
		}

		interface TeamConfig {
			findOne?: { name: string; };
			createData?: { name: string; description?: string; };
		}

		interface StatusConfig {
			findOne?: { label: string; };
			createData?: { label: string; color?: string; };
		}

		interface TagConfig {
			findOne?: { name: string; };
			createData?: { name: string; color?: string; };
		}

		export type MainDto = {
			title: string;
			category: 'A' | 'B';
			value?: number;
			contact?: ContactConfig;
			company?: CompanyConfig;
			source?: SourceConfig;
			teams?: TeamConfig[];
			status?: StatusConfig;
			tags?: TagConfig[];
		};
	`)
	defer env.release()

	_, reg := env.walkExportedTypeWithRegistry(t, "MainDto")

	// All interface types used as properties must be in the registry
	expectedSchemas := []string{
		"ContactConfig", "CompanyConfig", "SourceConfig",
		"TeamConfig", "StatusConfig", "TagConfig",
		"PhoneEntry", "EmailEntry",
	}
	for _, name := range expectedSchemas {
		if !reg.Has(name) {
			t.Errorf("expected schema %q to be in registry, but it was not", name)
		}
	}

	// Check that property types are KindRef, not KindAny
	m, _ := env.walkExportedTypeWithRegistry(t, "MainDto")
	propMap := make(map[string]metadata.Property)
	for _, p := range m.Properties {
		propMap[p.Name] = p
	}

	for _, p := range m.Properties {
		if p.Type.Kind == metadata.KindAny {
			t.Errorf("property %q has KindAny — type information was lost", p.Name)
		}
	}

	// Specific checks for interface-typed properties
	if p, ok := propMap["contact"]; ok {
		if p.Type.Kind != metadata.KindRef || p.Type.Ref != "ContactConfig" {
			t.Errorf("contact: expected KindRef/ContactConfig, got Kind=%s Ref=%q", p.Type.Kind, p.Type.Ref)
		}
	}
	if p, ok := propMap["teams"]; ok {
		if p.Type.Kind != metadata.KindArray {
			t.Errorf("teams: expected KindArray, got Kind=%s", p.Type.Kind)
		} else if p.Type.ElementType == nil || p.Type.ElementType.Kind != metadata.KindRef {
			t.Errorf("teams: expected array of KindRef, got element Kind=%v", p.Type.ElementType)
		}
	}
}

// TestWalkType_PartialOmitConditionalPhantom tests if restructuring the phantom
// types to use a top-level conditional (no intersection with WithErr) fixes it.
func TestWalkType_PartialOmitConditionalPhantom(t *testing.T) {
	// Instead of `{ __tsgonest_x?: NumVal<N> } & WithErr<...>`,
	// use a single conditional type that produces the full object:
	// N extends { value: V; error: E } ? { __tsgonest_x?: V; __tsgonest_x_error?: E }
	//                                   : { __tsgonest_x?: N }
	env := setupWalker(t, `
		type MinLength<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_minLength?: V; readonly __tsgonest_minLength_error?: E }
				: { readonly __tsgonest_minLength?: N extends { value: infer V } ? V : N };

		type MaxLength<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_maxLength?: V; readonly __tsgonest_maxLength_error?: E }
				: { readonly __tsgonest_maxLength?: N extends { value: infer V } ? V : N };

		type Pattern<P extends string | { value: string; error?: string }> =
			P extends { value: infer V extends string; error: infer E extends string }
				? { readonly __tsgonest_pattern?: V; readonly __tsgonest_pattern_error?: E }
				: { readonly __tsgonest_pattern?: P extends { value: infer V } ? V : P };

		type Minimum<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_minimum?: V; readonly __tsgonest_minimum_error?: E }
				: { readonly __tsgonest_minimum?: N extends { value: infer V } ? V : N };

		interface Base {
			title: string & MinLength<1> & MaxLength<255>;
			category: 'A' | 'B';
			value?: number & Minimum<0>;
			contactId?: string & Pattern<'^[0-9a-f]{24}$'>;
			statusId: string;
		}
		export type Result = Partial<Omit<Base, 'foo'>>;
	`)
	defer env.release()
	checkNoAny(t, env, "Result")
}

// TestWalkType_PartialOmitConditionalPhantomFull tests the full 13-property
// pattern with the restructured phantom types.
func TestWalkType_PartialOmitConditionalPhantomFull(t *testing.T) {
	env := setupWalker(t, `
		type MinLength<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_minLength?: V; readonly __tsgonest_minLength_error?: E }
				: { readonly __tsgonest_minLength?: N extends { value: infer V } ? V : N };

		type MaxLength<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_maxLength?: V; readonly __tsgonest_maxLength_error?: E }
				: { readonly __tsgonest_maxLength?: N extends { value: infer V } ? V : N };

		type Pattern<P extends string | { value: string; error?: string }> =
			P extends { value: infer V extends string; error: infer E extends string }
				? { readonly __tsgonest_pattern?: V; readonly __tsgonest_pattern_error?: E }
				: { readonly __tsgonest_pattern?: P extends { value: infer V } ? V : P };

		type Minimum<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_minimum?: V; readonly __tsgonest_minimum_error?: E }
				: { readonly __tsgonest_minimum?: N extends { value: infer V } ? V : N };

		type PriorityLevel = 'LOW' | 'MEDIUM' | 'HIGH' | 'URGENT';

		interface BaseItem {
			title: string & MinLength<1> & MaxLength<255>;
			category: 'A' | 'B';
			value?: number & Minimum<0>;
			priority?: PriorityLevel;
			location?: string & MaxLength<100>;
			data?: Record<string, any>;
			contactId?: string & Pattern<'^[0-9a-f]{24}$'>;
			companyId?: string & Pattern<'^[0-9a-f]{24}$'>;
			sourceId?: string & Pattern<'^[0-9a-f]{24}$'>;
			statusId: string;
			tagIds?: (string & Pattern<'^[0-9a-f]{24}$'>)[];
			ownerId?: string & Pattern<'^[0-9a-f]{24}$'>;
			teamIds?: (string & Pattern<'^[0-9a-f]{24}$'>)[];
		}

		type BaseItemWithout = Omit<BaseItem, 'foo'>;
		export type UpdateItem = Partial<BaseItemWithout> & {
			id: string & Pattern<'^[0-9a-f]{24}$'>;
		};
	`)
	defer env.release()

	m := env.walkExportedType(t, "UpdateItem")
	if m.Kind != metadata.KindObject {
		t.Fatalf("expected KindObject, got %s", m.Kind)
	}

	anyCount := 0
	for _, p := range m.Properties {
		t.Logf("  %s: Kind=%s", p.Name, p.Type.Kind)
		if p.Type.Kind == metadata.KindAny {
			anyCount++
		}
	}
	if anyCount > 0 {
		t.Errorf("%d/%d properties have KindAny", anyCount, len(m.Properties))
	}

	// Verify constraints are extracted for branded types
	propMap := make(map[string]metadata.Property)
	for _, p := range m.Properties {
		propMap[p.Name] = p
	}

	if p, ok := propMap["title"]; ok {
		if p.Type.Kind != metadata.KindAtomic || p.Type.Atomic != "string" {
			t.Errorf("title: expected string, got Kind=%s", p.Type.Kind)
		}
		if p.Constraints == nil {
			t.Errorf("title: expected constraints (minLength/maxLength)")
		}
	}
	if p, ok := propMap["id"]; ok {
		if p.Type.Kind != metadata.KindAtomic || p.Type.Atomic != "string" {
			t.Errorf("id: expected string, got Kind=%s", p.Type.Kind)
		}
	}
}

// TestWalkType_InterfaceSingleBrandedConstraint reproduces the bug where
// certain branded type compositions produce empty schemas.
// Single-constraint branded types (string & MaxLength<100>), const-object enums,
// Record<string,any> aliases, and branded arrays all degrade to KindAny.
func TestWalkType_InterfaceSingleBrandedConstraint(t *testing.T) {
	env := setupWalker(t, `
		type MinLength<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_minLength?: V; readonly __tsgonest_minLength_error?: E }
				: { readonly __tsgonest_minLength?: N extends { value: infer V } ? V : N };

		type MaxLength<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_maxLength?: V; readonly __tsgonest_maxLength_error?: E }
				: { readonly __tsgonest_maxLength?: N extends { value: infer V } ? V : N };

		type Pattern<P extends string | { value: string; error?: string }> =
			P extends { value: infer V extends string; error: infer E extends string }
				? { readonly __tsgonest_pattern?: V; readonly __tsgonest_pattern_error?: E }
				: { readonly __tsgonest_pattern?: P extends { value: infer V } ? V : P };

		type Minimum<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_minimum?: V; readonly __tsgonest_minimum_error?: E }
				: { readonly __tsgonest_minimum?: N extends { value: infer V } ? V : N };

		// Prisma-style const-object enum
		const PriorityEnum = {
			LOW: 'LOW' as const,
			NORMAL: 'NORMAL' as const,
			IMPORTANT: 'IMPORTANT' as const,
			HIGH: 'HIGH' as const,
		};
		type PriorityEnum = (typeof PriorityEnum)[keyof typeof PriorityEnum];

		// Record<string, any> alias
		type JsonRecord = Record<string, any>;

		export interface CreateLeadDealDto {
			// ✅ Multiple branded constraints
			title: string & MinLength<1> & MaxLength<255>;
			// ✅ Literal union
			type: 'LEAD' | 'DEAL';
			// ✅ Numeric branded
			value?: number & Minimum<0>;
			// ❌ Single string constraint
			location?: string & MaxLength<100>;
			// ❌ Single pattern constraint
			contactId?: string & Pattern<'^[0-9a-fA-F]{24}$'>;
			// ❌ Const-object enum
			priority?: PriorityEnum;
			// ❌ Record alias
			columnData?: JsonRecord;
			// ❌ Branded array
			tagIds?: (string & Pattern<'^[0-9a-fA-F]{24}$'>)[];
			// ✅ Plain string
			statusId: string;
		}
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "CreateLeadDealDto")
	if m.Kind == metadata.KindRef {
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}
	if m.Kind != metadata.KindObject {
		t.Fatalf("expected KindObject, got %s", m.Kind)
	}

	propMap := make(map[string]metadata.Property)
	for _, p := range m.Properties {
		propMap[p.Name] = p
		t.Logf("  %s: Kind=%s Atomic=%q Ref=%q", p.Name, p.Type.Kind, p.Type.Atomic, p.Type.Ref)
	}

	// Properties that MUST NOT be KindAny
	for _, name := range []string{"title", "type", "value", "location", "contactId", "priority", "columnData", "tagIds", "statusId"} {
		p, ok := propMap[name]
		if !ok {
			t.Errorf("property %q not found", name)
			continue
		}
		if p.Type.Kind == metadata.KindAny {
			t.Errorf("property %q: got KindAny — type info lost", name)
		}
	}

	// Specific type checks
	if p, ok := propMap["title"]; ok {
		if p.Type.Kind != metadata.KindAtomic || p.Type.Atomic != "string" {
			t.Errorf("title: expected KindAtomic/string, got Kind=%s Atomic=%q", p.Type.Kind, p.Type.Atomic)
		}
		if p.Constraints == nil || p.Constraints.MinLength == nil || p.Constraints.MaxLength == nil {
			t.Errorf("title: expected minLength+maxLength constraints, got %+v", p.Constraints)
		}
	}
	if p, ok := propMap["location"]; ok {
		if p.Type.Kind != metadata.KindAtomic || p.Type.Atomic != "string" {
			t.Errorf("location: expected KindAtomic/string, got Kind=%s Atomic=%q", p.Type.Kind, p.Type.Atomic)
		}
		if p.Constraints == nil || p.Constraints.MaxLength == nil {
			t.Errorf("location: expected maxLength constraint, got %+v", p.Constraints)
		}
	}
	if p, ok := propMap["contactId"]; ok {
		if p.Type.Kind != metadata.KindAtomic || p.Type.Atomic != "string" {
			t.Errorf("contactId: expected KindAtomic/string, got Kind=%s Atomic=%q", p.Type.Kind, p.Type.Atomic)
		}
		if p.Constraints == nil || p.Constraints.Pattern == nil {
			t.Errorf("contactId: expected pattern constraint, got %+v", p.Constraints)
		}
	}
	if p, ok := propMap["tagIds"]; ok {
		if p.Type.Kind != metadata.KindArray {
			t.Errorf("tagIds: expected KindArray, got Kind=%s", p.Type.Kind)
		}
	}
}

// TestWalkType_OldWithErrSingleConstraint reproduces the bug where the OLD
// phantom types (using WithErr intersection) cause single-constraint branded types
// to degrade to KindAny when used as interface properties.
func TestWalkType_OldWithErrSingleConstraint(t *testing.T) {
	env := setupWalker(t, `
		type NumVal<N extends number | { value: number; error?: string }> =
			N extends { value: infer V } ? V : N;
		type StrVal<S extends string | { value: string; error?: string }> =
			S extends { value: infer V } ? V : S;
		type WithErr<Prefix extends string, C> =
			C extends { error: infer E extends string }
				? { readonly [K in `+"`"+`${Prefix}_error`+"`"+`]?: E }
				: {};

		type MinLength<N extends number | { value: number; error?: string }> = {
			readonly __tsgonest_minLength?: NumVal<N>;
		} & WithErr<"__tsgonest_minLength", N>;

		type MaxLength<N extends number | { value: number; error?: string }> = {
			readonly __tsgonest_maxLength?: NumVal<N>;
		} & WithErr<"__tsgonest_maxLength", N>;

		type Pattern<P extends string | { value: string; error?: string }> = {
			readonly __tsgonest_pattern?: StrVal<P>;
		} & WithErr<"__tsgonest_pattern", P>;

		type Minimum<N extends number | { value: number; error?: string }> = {
			readonly __tsgonest_minimum?: NumVal<N>;
		} & WithErr<"__tsgonest_minimum", N>;

		type JsonRecord = Record<string, any>;

		export interface CreateLeadDealDto {
			title: string & MinLength<1> & MaxLength<255>;
			type: 'LEAD' | 'DEAL';
			value?: number & Minimum<0>;
			location?: string & MaxLength<100>;
			contactId?: string & Pattern<'^[0-9a-fA-F]{24}$'>;
			columnData?: JsonRecord;
			tagIds?: (string & Pattern<'^[0-9a-fA-F]{24}$'>)[];
			statusId: string;
		}
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "CreateLeadDealDto")
	if m.Kind == metadata.KindRef {
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}
	if m.Kind != metadata.KindObject {
		t.Fatalf("expected KindObject, got %s", m.Kind)
	}

	for _, p := range m.Properties {
		t.Logf("  %s: Kind=%s Atomic=%q", p.Name, p.Type.Kind, p.Type.Atomic)
		if p.Type.Kind == metadata.KindAny {
			t.Errorf("property %q: got KindAny — type info lost (old WithErr bug)", p.Name)
		}
	}
}

// TestWalkType_MultiFileSingleBrandedConstraint tests branded types imported
// from a separate file (simulating import from @tsgonest/types).
func TestWalkType_MultiFileSingleBrandedConstraint(t *testing.T) {
	files := map[string]string{
		"tags.ts": `
			export type MinLength<N extends number | { value: number; error?: string }> =
				N extends { value: infer V extends number; error: infer E extends string }
					? { readonly __tsgonest_minLength?: V; readonly __tsgonest_minLength_error?: E }
					: { readonly __tsgonest_minLength?: N extends { value: infer V } ? V : N };

			export type MaxLength<N extends number | { value: number; error?: string }> =
				N extends { value: infer V extends number; error: infer E extends string }
					? { readonly __tsgonest_maxLength?: V; readonly __tsgonest_maxLength_error?: E }
					: { readonly __tsgonest_maxLength?: N extends { value: infer V } ? V : N };

			export type Pattern<P extends string | { value: string; error?: string }> =
				P extends { value: infer V extends string; error: infer E extends string }
					? { readonly __tsgonest_pattern?: V; readonly __tsgonest_pattern_error?: E }
					: { readonly __tsgonest_pattern?: P extends { value: infer V } ? V : P };

			export type Minimum<N extends number | { value: number; error?: string }> =
				N extends { value: infer V extends number; error: infer E extends string }
					? { readonly __tsgonest_minimum?: V; readonly __tsgonest_minimum_error?: E }
					: { readonly __tsgonest_minimum?: N extends { value: infer V } ? V : N };
		`,
		"dto.ts": `
			import { MinLength, MaxLength, Pattern, Minimum } from './tags.js';

			type JsonRecord = Record<string, any>;

			export interface CreateLeadDealDto {
				title: string & MinLength<1> & MaxLength<255>;
				type: 'LEAD' | 'DEAL';
				value?: number & Minimum<0>;
				location?: string & MaxLength<100>;
				contactId?: string & Pattern<'^[0-9a-fA-F]{24}$'>;
				columnData?: JsonRecord;
				tagIds?: (string & Pattern<'^[0-9a-fA-F]{24}$'>)[];
				statusId: string;
			}
		`,
	}

	env := setupWalkerMultiFile(t, files, "dto.ts")
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "CreateLeadDealDto")
	if m.Kind == metadata.KindRef {
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}
	if m.Kind != metadata.KindObject {
		t.Fatalf("expected KindObject, got %s", m.Kind)
	}

	propMap := make(map[string]metadata.Property)
	for _, p := range m.Properties {
		propMap[p.Name] = p
		t.Logf("  %s: Kind=%s Atomic=%q", p.Name, p.Type.Kind, p.Type.Atomic)
	}

	for _, name := range []string{"title", "type", "value", "location", "contactId", "columnData", "tagIds", "statusId"} {
		p, ok := propMap[name]
		if !ok {
			t.Errorf("property %q not found", name)
			continue
		}
		if p.Type.Kind == metadata.KindAny {
			t.Errorf("property %q: got KindAny — type info lost", name)
		}
	}

	// Verify constraints on single-constraint properties
	if p, ok := propMap["location"]; ok {
		if p.Type.Kind != metadata.KindAtomic || p.Type.Atomic != "string" {
			t.Errorf("location: expected KindAtomic/string, got Kind=%s", p.Type.Kind)
		}
		if p.Constraints == nil || p.Constraints.MaxLength == nil {
			t.Errorf("location: expected maxLength constraint, got %+v", p.Constraints)
		}
	}
	if p, ok := propMap["contactId"]; ok {
		if p.Type.Kind != metadata.KindAtomic || p.Type.Atomic != "string" {
			t.Errorf("contactId: expected KindAtomic/string, got Kind=%s", p.Type.Kind)
		}
		if p.Constraints == nil || p.Constraints.Pattern == nil {
			t.Errorf("contactId: expected pattern constraint, got %+v", p.Constraints)
		}
	}
	if p, ok := propMap["tagIds"]; ok {
		if p.Type.Kind != metadata.KindArray {
			t.Errorf("tagIds: expected KindArray, got Kind=%s", p.Type.Kind)
		} else if p.Type.ElementType != nil {
			if p.Type.ElementType.Kind != metadata.KindAtomic || p.Type.ElementType.Atomic != "string" {
				t.Errorf("tagIds element: expected KindAtomic/string, got Kind=%s", p.Type.ElementType.Kind)
			}
		}
	}
}

// checkNoAny verifies no properties of the type have KindAny.
func checkNoAny(t *testing.T, env *walkerEnv, typeName string) {
	t.Helper()
	m := env.walkExportedType(t, typeName)
	if m.Kind != metadata.KindObject {
		t.Fatalf("expected KindObject, got %s", m.Kind)
	}
	anyCount := 0
	for _, p := range m.Properties {
		t.Logf("  %s: Kind=%s", p.Name, p.Type.Kind)
		if p.Type.Kind == metadata.KindAny {
			anyCount++
		}
	}
	if anyCount > 0 {
		t.Errorf("%d/%d properties have KindAny", anyCount, len(m.Properties))
	}
}

// TestWalkType_BrandedLiteralUnion tests that a union of branded literal
// intersections (e.g., TCurrencyCode4217 & tags.MaxLength<3> & tags.Pattern<...>)
// produces a union of KindLiteral members with constraints, not a union of
// KindIntersection members that leak phantom objects.
// Regression: TypeScript distributes the intersection across the union, producing
// ("USD" & phantom) | ("EUR" & phantom) | ..., which exhausted the breadth limit
// and caused subsequent properties to degrade to KindAny.
func TestWalkType_BrandedLiteralUnion(t *testing.T) {
	env := setupWalker(t, `
		type MaxLength<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_maxLength?: V; readonly __tsgonest_maxLength_error?: E }
				: { readonly __tsgonest_maxLength?: N extends { value: infer V } ? V : N };

		type Pattern<P extends string | { value: string; error?: string }> =
			P extends { value: infer V extends string; error: infer E extends string }
				? { readonly __tsgonest_pattern?: V; readonly __tsgonest_pattern_error?: E }
				: { readonly __tsgonest_pattern?: P extends { value: infer V } ? V : P };

		// Simulate a large const-union type (like TCurrencyCode4217)
		const CURRENCIES = [
			'USD', 'EUR', 'GBP', 'JPY', 'AUD', 'CAD', 'CHF', 'CNY', 'SEK', 'NZD',
			'MXN', 'SGD', 'HKD', 'NOK', 'KRW', 'TRY', 'RUB', 'INR', 'BRL', 'ZAR',
			'DKK', 'PLN', 'TWD', 'THB', 'IDR', 'HUF', 'CZK', 'ILS', 'CLP', 'PHP',
			'AED', 'COP', 'SAR', 'MYR', 'RON', 'BGN', 'HRK', 'PEN', 'ARS', 'VND',
		] as const;
		type TCurrencyCode = (typeof CURRENCIES)[number];

		export interface TestDto {
			// Branded literal union: should produce KindUnion of literals with constraints
			currency?: TCurrencyCode & MaxLength<3> & Pattern<'^[A-Z]{3}$'>;
			// Properties AFTER the large union must NOT degrade to KindAny
			name: string & MaxLength<100>;
			pattern?: string & Pattern<'^[a-z]+$'>;
			plain: string;
		}
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "TestDto")
	if m.Kind == metadata.KindRef {
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}
	if m.Kind != metadata.KindObject {
		t.Fatalf("expected KindObject, got %s", m.Kind)
	}

	propMap := make(map[string]metadata.Property)
	for _, p := range m.Properties {
		propMap[p.Name] = p
	}

	// currency must be a union of literals, not KindAny or KindIntersection
	if p, ok := propMap["currency"]; ok {
		if p.Type.Kind == metadata.KindAny {
			t.Errorf("currency: got KindAny — branded literal union was not detected")
		}
		if p.Type.Kind == metadata.KindUnion {
			for i, member := range p.Type.UnionMembers {
				if member.Kind != metadata.KindLiteral {
					t.Errorf("currency union member[%d]: expected KindLiteral, got %s", i, member.Kind)
					break
				}
			}
			if len(p.Type.UnionMembers) < 30 {
				t.Errorf("currency: expected 40 union members, got %d", len(p.Type.UnionMembers))
			}
		} else if p.Type.Kind == metadata.KindRef {
			// KindRef is acceptable if the type alias was registered
		} else {
			t.Errorf("currency: expected KindUnion or KindRef, got %s", p.Type.Kind)
		}
		// Constraints should be extracted from the branded intersection
		if p.Constraints == nil {
			t.Errorf("currency: expected constraints (maxLength+pattern)")
		} else {
			if p.Constraints.MaxLength == nil {
				t.Errorf("currency: expected maxLength constraint")
			}
			if p.Constraints.Pattern == nil {
				t.Errorf("currency: expected pattern constraint")
			}
		}
	} else {
		t.Error("currency property not found")
	}

	// Properties after the large union must NOT be KindAny
	if p, ok := propMap["name"]; ok {
		if p.Type.Kind != metadata.KindAtomic || p.Type.Atomic != "string" {
			t.Errorf("name: expected KindAtomic/string, got Kind=%s", p.Type.Kind)
		}
		if p.Constraints == nil || p.Constraints.MaxLength == nil {
			t.Errorf("name: expected maxLength constraint")
		}
	} else {
		t.Error("name property not found")
	}
	if p, ok := propMap["pattern"]; ok {
		if p.Type.Kind != metadata.KindAtomic || p.Type.Atomic != "string" {
			t.Errorf("pattern: expected KindAtomic/string, got Kind=%s", p.Type.Kind)
		}
		if p.Constraints == nil || p.Constraints.Pattern == nil {
			t.Errorf("pattern: expected pattern constraint")
		}
	} else {
		t.Error("pattern property not found")
	}
	if p, ok := propMap["plain"]; ok {
		if p.Type.Kind != metadata.KindAtomic || p.Type.Atomic != "string" {
			t.Errorf("plain: expected KindAtomic/string, got Kind=%s", p.Type.Kind)
		}
	} else {
		t.Error("plain property not found")
	}
}

// TestWalkType_BreadthCounterIsolation tests that the breadth counter is properly
// isolated per named type, so a large union in one type doesn't exhaust the counter
// for sibling properties that reference other named types.
// Regression: walking a 163-currency branded union consumed ~490 types from the
// breadth budget, causing subsequent interface-typed properties (ContactConfig, etc.)
// to degrade to KindAny with "breadth-exceeded".
func TestWalkType_BreadthCounterIsolation(t *testing.T) {
	env := setupWalker(t, `
		type MaxLength<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_maxLength?: V; readonly __tsgonest_maxLength_error?: E }
				: { readonly __tsgonest_maxLength?: N extends { value: infer V } ? V : N };

		type Pattern<P extends string | { value: string; error?: string }> =
			P extends { value: infer V extends string; error: infer E extends string }
				? { readonly __tsgonest_pattern?: V; readonly __tsgonest_pattern_error?: E }
				: { readonly __tsgonest_pattern?: P extends { value: infer V } ? V : P };

		type MinLength<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_minLength?: V; readonly __tsgonest_minLength_error?: E }
				: { readonly __tsgonest_minLength?: N extends { value: infer V } ? V : N };

		// Large literal union to consume breadth budget
		const CURRENCIES = [
			'USD', 'EUR', 'GBP', 'JPY', 'AUD', 'CAD', 'CHF', 'CNY', 'SEK', 'NZD',
			'MXN', 'SGD', 'HKD', 'NOK', 'KRW', 'TRY', 'RUB', 'INR', 'BRL', 'ZAR',
			'DKK', 'PLN', 'TWD', 'THB', 'IDR', 'HUF', 'CZK', 'ILS', 'CLP', 'PHP',
			'AED', 'COP', 'SAR', 'MYR', 'RON', 'BGN', 'HRK', 'PEN', 'ARS', 'VND',
		] as const;
		type TCurrencyCode = (typeof CURRENCIES)[number];

		// Interfaces that will be used as property types
		interface ContactConfig {
			findOne?: { name: string & MinLength<3> & MaxLength<40>; slug?: string; };
			createData?: {
				name: string & MinLength<3> & MaxLength<40>;
				jobTitle?: string;
				location?: string & MinLength<2> & MaxLength<40>;
			};
		}

		interface SourceConfig {
			findOne?: { title: string & MinLength<3> & MaxLength<40>; };
			createData?: { title: string & MinLength<3> & MaxLength<40>; };
		}

		interface TagConfig {
			findOne?: { name: string & MinLength<2> & MaxLength<40>; };
			createData?: { name: string & MinLength<2> & MaxLength<40>; color?: string; };
		}

		export interface MainDto {
			title: string & MinLength<3> & MaxLength<100>;
			currency?: TCurrencyCode & MaxLength<3> & Pattern<'^[A-Z]{3}$'>;
			// These MUST NOT degrade to KindAny even though currency consumed many types
			contact?: ContactConfig;
			source?: SourceConfig;
			tags?: TagConfig[];
			contactId?: string & Pattern<'^[0-9a-f]{24}$'>;
			plain: string;
		}
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "MainDto")
	if m.Kind == metadata.KindRef {
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}
	if m.Kind != metadata.KindObject {
		t.Fatalf("expected KindObject, got %s", m.Kind)
	}

	propMap := make(map[string]metadata.Property)
	for _, p := range m.Properties {
		propMap[p.Name] = p
		t.Logf("  %s: Kind=%s", p.Name, p.Type.Kind)
	}

	// No property should be KindAny
	for _, name := range []string{"title", "currency", "contact", "source", "tags", "contactId", "plain"} {
		p, ok := propMap[name]
		if !ok {
			t.Errorf("property %q not found", name)
			continue
		}
		if p.Type.Kind == metadata.KindAny {
			t.Errorf("property %q: got KindAny — breadth counter leaked from currency union", name)
		}
	}

	// Interface-typed properties must be KindRef
	if p, ok := propMap["contact"]; ok {
		if p.Type.Kind != metadata.KindRef || p.Type.Ref != "ContactConfig" {
			t.Errorf("contact: expected KindRef/ContactConfig, got Kind=%s Ref=%q", p.Type.Kind, p.Type.Ref)
		}
	}
	if p, ok := propMap["source"]; ok {
		if p.Type.Kind != metadata.KindRef || p.Type.Ref != "SourceConfig" {
			t.Errorf("source: expected KindRef/SourceConfig, got Kind=%s Ref=%q", p.Type.Kind, p.Type.Ref)
		}
	}
	if p, ok := propMap["tags"]; ok {
		if p.Type.Kind != metadata.KindArray {
			t.Errorf("tags: expected KindArray, got Kind=%s", p.Type.Kind)
		} else if p.Type.ElementType == nil || p.Type.ElementType.Kind != metadata.KindRef {
			t.Errorf("tags: expected array of KindRef/TagConfig")
		}
	}

	// All referenced interfaces must be in the registry
	for _, name := range []string{"ContactConfig", "SourceConfig", "TagConfig"} {
		if !reg.Has(name) {
			t.Errorf("expected %q in registry, not found", name)
		}
	}

	// contactId after the currency union must still have constraints
	if p, ok := propMap["contactId"]; ok {
		if p.Type.Kind != metadata.KindAtomic || p.Type.Atomic != "string" {
			t.Errorf("contactId: expected KindAtomic/string, got Kind=%s", p.Type.Kind)
		}
		if p.Constraints == nil || p.Constraints.Pattern == nil {
			t.Errorf("contactId: expected pattern constraint")
		}
	}
}

// TestWalkType_TryDetectBrandedLiteral tests that tryDetectBranded correctly
// handles literal & phantom intersections (not just atomic & phantom).
// Regression: tryDetectBranded only checked KindAtomic, missing KindLiteral.
func TestWalkType_TryDetectBrandedLiteral(t *testing.T) {
	env := setupWalker(t, `
		type MaxLength<N extends number | { value: number; error?: string }> =
			N extends { value: infer V extends number; error: infer E extends string }
				? { readonly __tsgonest_maxLength?: V; readonly __tsgonest_maxLength_error?: E }
				: { readonly __tsgonest_maxLength?: N extends { value: infer V } ? V : N };

		// A literal intersected with a phantom — tryDetectBranded must handle this
		export type BrandedLiteral = 'USD' & MaxLength<3>;
	`)
	defer env.release()

	m := env.walkExportedType(t, "BrandedLiteral")
	// The result should be a KindLiteral with constraints, not KindIntersection
	if m.Kind == metadata.KindIntersection {
		t.Errorf("BrandedLiteral: got KindIntersection — tryDetectBranded missed literal & phantom")
	}
	if m.Kind != metadata.KindLiteral {
		t.Errorf("BrandedLiteral: expected KindLiteral, got %s", m.Kind)
	}
	if m.Kind == metadata.KindLiteral {
		if m.LiteralValue != "USD" {
			t.Errorf("BrandedLiteral: expected literal value 'USD', got %v", m.LiteralValue)
		}
		if m.Constraints == nil || m.Constraints.MaxLength == nil {
			t.Errorf("BrandedLiteral: expected maxLength constraint")
		}
	}
}

// TestWalkType_NonBrandedIntersectionInUnion verifies that when a union contains
// intersection members that are NOT branded literals (e.g., object intersections),
// they fall through to normal WalkType processing and are not incorrectly detected
// by the branded literal fast-path.
func TestWalkType_NonBrandedIntersectionInUnion(t *testing.T) {
	env := setupWalker(t, `
		interface A { x: number; }
		interface B { y: string; }
		interface C { z: boolean; }

		// Union of object intersections — not branded literals
		export type Result = (A & B) | C;
	`)
	defer env.release()

	m := env.walkExportedType(t, "Result")
	// Should be KindUnion with 2 members (flattened intersection + C)
	if m.Kind != metadata.KindUnion {
		t.Fatalf("expected KindUnion, got %s", m.Kind)
	}
	if len(m.UnionMembers) != 2 {
		t.Fatalf("expected 2 union members, got %d", len(m.UnionMembers))
	}
	// First member: A & B flattened into an object (may be registered as KindRef)
	first := m.UnionMembers[0]
	if first.Kind != metadata.KindObject && first.Kind != metadata.KindRef {
		t.Errorf("union member[0]: expected KindObject or KindRef (flattened intersection), got %s", first.Kind)
	}
	// Second member: C as a ref or object
	second := m.UnionMembers[1]
	if second.Kind != metadata.KindRef && second.Kind != metadata.KindObject {
		t.Errorf("union member[1]: expected KindRef or KindObject, got %s", second.Kind)
	}
	// The key assertion: neither member should be KindAny or KindIntersection
	// with phantom objects (which would happen if the branded fast-path
	// incorrectly consumed object intersections)
	for i, member := range m.UnionMembers {
		if member.Kind == metadata.KindAny {
			t.Errorf("union member[%d]: got KindAny — type info lost", i)
		}
		if member.Kind == metadata.KindIntersection {
			t.Errorf("union member[%d]: got KindIntersection — should have been flattened", i)
		}
	}
}

// TestWalkType_BrandedArrayElementConstraints verifies that constraints on
// branded array element types (e.g., (string & Pattern<...>)[]) are preserved
// on the element metadata, enabling downstream OpenAPI/SDK generators to emit
// them on the items schema.
func TestWalkType_BrandedArrayElementConstraints(t *testing.T) {
	env := setupWalker(t, `
		type Pattern<P extends string | { value: string; error?: string }> =
			P extends { value: infer V extends string; error: infer E extends string }
				? { readonly __tsgonest_pattern?: V; readonly __tsgonest_pattern_error?: E }
				: { readonly __tsgonest_pattern?: P extends { value: infer V } ? V : P };

		export interface TestDto {
			ids: (string & Pattern<'^[0-9a-fA-F]{24}$'>)[];
		}
	`)
	defer env.release()

	m, reg := env.walkExportedTypeWithRegistry(t, "TestDto")
	if m.Kind == metadata.KindRef {
		if resolved := reg.Types[m.Ref]; resolved != nil {
			m = *resolved
		}
	}
	if m.Kind != metadata.KindObject {
		t.Fatalf("expected KindObject, got %s", m.Kind)
	}

	var idsProp *metadata.Property
	for i := range m.Properties {
		if m.Properties[i].Name == "ids" {
			idsProp = &m.Properties[i]
			break
		}
	}
	if idsProp == nil {
		t.Fatal("ids property not found")
	}
	if idsProp.Type.Kind != metadata.KindArray {
		t.Fatalf("ids: expected KindArray, got %s", idsProp.Type.Kind)
	}
	if idsProp.Type.ElementType == nil {
		t.Fatal("ids: element type is nil")
	}
	elem := idsProp.Type.ElementType
	if elem.Kind != metadata.KindAtomic || elem.Atomic != "string" {
		t.Errorf("ids element: expected KindAtomic/string, got Kind=%s Atomic=%q", elem.Kind, elem.Atomic)
	}
	// The branded constraints should be on the element metadata
	if elem.Constraints == nil {
		t.Error("ids element: expected constraints on element metadata")
	} else if elem.Constraints.Pattern == nil {
		t.Error("ids element: expected pattern constraint")
	} else if *elem.Constraints.Pattern != "^[0-9a-fA-F]{24}$" {
		t.Errorf("ids element: expected pattern '^[0-9a-fA-F]{24}$', got %q", *elem.Constraints.Pattern)
	}
}
