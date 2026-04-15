// Simulates @tiptap/core's JSONContent — a self-recursive interface.
// This file intentionally does not define a controller and is not listed
// in controllers.include, so JSONContent never gets walked at the top level.
// It is only reached transitively from the DTO that references it, which
// is exactly the case that previously caused tsgonest to emit unresolved
// isJSONContent / validateJSONContent / serializeJSONContent calls.
export interface JSONContent {
  type?: string;
  attrs?: Record<string, unknown>;
  content?: JSONContent[];
  marks?: Array<{ type: string; [k: string]: unknown }>;
  text?: string;
}
