import { Token } from './token'
import {
  type Provider,
  type Scope,
  isClassProvider,
  isValueProvider,
  isFactoryProvider,
  providerToken,
} from './module'
import { topoSort } from './topological'

export type Constructor<T = unknown> = new (...args: any[]) => T
export type Factory<T> = (container: Container) => T

type TokenKey = Token<any> | Constructor<any>

interface Registration {
  token: TokenKey
  provider: Provider
  declaringModule?: object
  scope: Scope
  deps: TokenKey[]
}

/**
 * DI container. Phase 1 exposed `register(token, factory)` for direct singleton
 * binding; Phases 3+ add `registerProvider(provider, declaringModule?)` plus a
 * `boot()` phase that resolves dependencies in topological order and supports
 * `useValue` / `useFactory` (sync or async) providers.
 *
 * Class-constructor deps are read from `static inject = [TokenA, TokenB] as const`.
 * Reflect-metadata is intentionally not consulted — runtime stays zero-dep.
 */
export class Container {
  private readonly registrations = new Map<TokenKey, Registration>()
  private readonly instances = new Map<TokenKey, unknown>()
  private readonly legacyFactories = new Map<TokenKey, Factory<unknown>>()
  private booted = false
  private constructionOrder: unknown[] = []

  register<T>(provider: Token<T> | Constructor<T>, factory: Factory<T>): void {
    this.legacyFactories.set(provider, factory as Factory<unknown>)
  }

  /**
   * High-level provider registration. Throws on duplicate registration for the
   * same token. `declaringModule` is opaque metadata used by the boot pipeline
   * for visibility validation; the container itself does not interpret it.
   */
  registerProvider(provider: Provider, declaringModule?: object): void {
    const token = providerToken(provider)
    if (this.registrations.has(token) || this.legacyFactories.has(token)) {
      throw new Error(`duplicate provider registration for ${describeToken(token)}`)
    }
    const scope: Scope = isFactoryProvider(provider)
      ? (provider.scope ?? 'singleton')
      : 'singleton'
    this.registrations.set(token, {
      token,
      provider,
      declaringModule,
      scope,
      deps: providerDeps(provider),
    })
  }

  /**
   * Resolves dependencies in topological order, constructing singletons and
   * awaiting async factories. Idempotent — second call is a no-op.
   */
  async boot(): Promise<void> {
    if (this.booted) return

    for (const reg of this.registrations.values()) {
      for (const dep of reg.deps) {
        if (!this.registrations.has(dep) && !this.legacyFactories.has(dep)) {
          throw new Error(
            `missing provider for ${describeToken(dep)} (required by ${describeToken(reg.token)})`,
          )
        }
      }
    }

    const order = topoSort(
      [...this.registrations.values()],
      (reg) =>
        reg.deps
          .map((d) => this.registrations.get(d))
          .filter((d): d is Registration => d !== undefined),
      {
        id: (reg) => reg.token,
        label: (reg) => describeToken(reg.token),
      },
    )

    for (const reg of order) {
      if (reg.scope === 'transient' && isFactoryProvider(reg.provider)) continue
      const value = await this.construct(reg)
      this.instances.set(reg.token, value)
      this.constructionOrder.push(value)
    }

    this.booted = true
  }

  /** Reverse construction order — what dispose walks in. */
  disposalOrder(): unknown[] {
    return [...this.constructionOrder].reverse()
  }

  /** The list of constructed instances, in topological order. */
  initOrder(): unknown[] {
    return [...this.constructionOrder]
  }

  /**
   * Resolve a token. Post-boot:
   *   singleton → cache hit.
   *   transient factory → re-runs the factory each call (sync only post-boot).
   * Legacy `register(token, factory)` bindings are cached on first hit.
   */
  resolve<T>(provider: Token<T> | Constructor<T>): T {
    if (this.instances.has(provider)) return this.instances.get(provider) as T

    const reg = this.registrations.get(provider)
    if (reg && reg.scope === 'transient' && isFactoryProvider(reg.provider)) {
      const args = reg.deps.map((d) => this.resolve(d))
      const v = reg.provider.useFactory(...args)
      if (v instanceof Promise) {
        throw new Error(
          `transient factory for ${describeToken(provider)} returned a Promise; transient resolution is synchronous`,
        )
      }
      return v as T
    }

    const legacy = this.legacyFactories.get(provider)
    if (legacy) {
      const v = legacy(this) as T
      this.instances.set(provider, v)
      return v
    }

    throw new Error(`No provider registered for ${describeToken(provider)}`)
  }

  has(provider: Token<any> | Constructor<any>): boolean {
    return (
      this.registrations.has(provider) ||
      this.legacyFactories.has(provider) ||
      this.instances.has(provider)
    )
  }

  registeredTokens(): TokenKey[] {
    return [...this.registrations.keys()]
  }

  getRegistration(token: TokenKey): Registration | undefined {
    return this.registrations.get(token)
  }

  private async construct(reg: Registration): Promise<unknown> {
    const { provider } = reg
    if (isValueProvider(provider)) return provider.useValue
    if (isFactoryProvider(provider)) {
      const args = reg.deps.map((d) => this.resolveBoot(d))
      const v = await provider.useFactory(...args)
      return v
    }
    if (isClassProvider(provider)) {
      const args = reg.deps.map((d) => this.resolveBoot(d))
      return new provider(...args)
    }
    throw new Error(`unknown provider shape: ${describeToken(reg.token)}`)
  }

  private resolveBoot(token: TokenKey): unknown {
    if (this.instances.has(token)) return this.instances.get(token)
    const legacy = this.legacyFactories.get(token)
    if (legacy) {
      const v = legacy(this)
      this.instances.set(token, v)
      return v
    }
    throw new Error(
      `provider ${describeToken(token)} requested before construction; topological sort invariant broken`,
    )
  }
}

function providerDeps(p: Provider): TokenKey[] {
  if (isClassProvider(p)) {
    const inject = (p as Constructor & { inject?: ReadonlyArray<TokenKey> }).inject
    return inject ? [...inject] : []
  }
  if (isFactoryProvider(p)) return p.inject ? [...p.inject] : []
  return []
}

export function describeToken(token: TokenKey): string {
  if (token instanceof Token) return `Token(${token.name})`
  return (token as Constructor).name || 'anonymous'
}
