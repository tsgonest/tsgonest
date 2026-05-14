/**
 * Topological sort with cycle detection. Nodes are keyed by reference identity
 * via the `id` callback (default: the node itself). Returns the topologically
 * ordered list (deps before dependents).
 *
 * On cycle, throws an Error whose message includes the cycle path as
 * `A -> B -> A`, using the `label` callback to format each node (default:
 * `String(node)`).
 */
export function topoSort<N>(
  nodes: readonly N[],
  deps: (node: N) => readonly N[],
  opts?: {
    id?: (node: N) => unknown
    label?: (node: N) => string
  },
): N[] {
  const id = opts?.id ?? ((n: N): unknown => n)
  const label = opts?.label ?? ((n: N): string => String(n))

  const byId = new Map<unknown, N>()
  for (const n of nodes) byId.set(id(n), n)

  const result: N[] = []
  const visited = new Set<unknown>()
  const onStack = new Set<unknown>()
  const stack: N[] = []

  const visit = (node: N): void => {
    const nid = id(node)
    if (visited.has(nid)) return
    if (onStack.has(nid)) {
      const startIdx = stack.findIndex((s) => id(s) === nid)
      const cycle = [...stack.slice(startIdx), node].map(label).join(' -> ')
      throw new Error(`Provider cycle detected: ${cycle}`)
    }
    onStack.add(nid)
    stack.push(node)
    for (const d of deps(node)) {
      const target = byId.get(id(d)) ?? d
      visit(target)
    }
    stack.pop()
    onStack.delete(nid)
    visited.add(nid)
    result.push(node)
  }

  for (const n of nodes) visit(n)
  return result
}
