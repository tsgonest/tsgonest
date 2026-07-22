import { assert } from "tsgonest";

type Foo = { a?: number; b: string };

export function toThing(value: unknown): Foo {
  return assert<Foo>(value);
}

console.log("valid:" + JSON.stringify(toThing({ b: "hi" })));
try {
  toThing({ a: "nope" });
  console.log("invalid:accepted");
} catch {
  console.log("invalid:rejected");
}
