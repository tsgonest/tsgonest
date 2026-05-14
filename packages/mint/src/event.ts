import type { Container, Constructor } from './container'
import { Token } from './token'

export interface EventResponseDraft {
  headers: Headers
  status: number
}

/**
 * Per-request execution context. Carries the incoming `Request`, a mutable
 * response draft for middleware to influence headers/status, and a small
 * token-store backed by a `Map`.
 */
export class Event {
  readonly request: Request
  readonly response: EventResponseDraft
  private readonly container: Container
  private readonly store = new Map<Token<unknown>, unknown>()

  constructor(request: Request, container: Container) {
    this.request = request
    this.container = container
    this.response = { headers: new Headers(), status: 200 }
  }

  resolve<T>(provider: Token<T> | Constructor<T>): T {
    return this.container.resolve(provider)
  }

  set<T>(token: Token<T>, value: T): void {
    this.store.set(token as Token<unknown>, value)
  }

  get<T>(token: Token<T>): T | undefined {
    return this.store.get(token as Token<unknown>) as T | undefined
  }

  require<T>(token: Token<T>): T {
    if (!this.store.has(token as Token<unknown>)) {
      throw new Error(`Event store has no entry for Token(${token.name})`)
    }
    return this.store.get(token as Token<unknown>) as T
  }
}
