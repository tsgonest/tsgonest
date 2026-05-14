/**
 * Structural lifecycle interfaces. Providers can implement either or both;
 * the container duck-types `init` / `Symbol.asyncDispose` at runtime.
 */
export interface OnInit {
  init(): void | Promise<void>
}

export function hasInit(value: unknown): value is OnInit {
  return (
    typeof value === 'object' &&
    value !== null &&
    typeof (value as { init?: unknown }).init === 'function'
  )
}

export function hasAsyncDispose(value: unknown): value is AsyncDisposable {
  return (
    typeof value === 'object' &&
    value !== null &&
    typeof (value as { [Symbol.asyncDispose]?: unknown })[Symbol.asyncDispose] ===
      'function'
  )
}

/**
 * Runs each disposer in order; collects thrown errors. If exactly one disposer
 * throws, rethrows that error; if more than one, throws AggregateError.
 */
export async function disposeAll(disposables: readonly AsyncDisposable[]): Promise<void> {
  const errors: unknown[] = []
  for (const d of disposables) {
    try {
      await d[Symbol.asyncDispose]()
    } catch (e) {
      errors.push(e)
    }
  }
  if (errors.length === 1) throw errors[0]
  if (errors.length > 1) throw new AggregateError(errors, 'multiple disposers failed')
}
