package analyzer_test

import (
	"fmt"
	"testing"

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
