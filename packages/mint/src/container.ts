import { Token } from './token'

export type Constructor<T = unknown> = new (...args: any[]) => T
export type Provider<T> = Token<T> | Constructor<T>
export type Factory<T> = (container: Container) => T

/**
 * Singleton-per-token DI container. Tokens can be either {@link Token}
 * instances or class constructors — lookup is by reference identity.
 */
export class Container {
  private readonly factories = new Map<unknown, Factory<unknown>>()
  private readonly instances = new Map<unknown, unknown>()

  register<T>(provider: Provider<T>, factory: Factory<T>): void {
    this.factories.set(provider, factory as Factory<unknown>)
  }

  resolve<T>(provider: Provider<T>): T {
    if (this.instances.has(provider)) {
      return this.instances.get(provider) as T
    }
    const factory = this.factories.get(provider)
    if (!factory) {
      const name =
        provider instanceof Token
          ? `Token(${provider.name})`
          : (provider as Constructor).name || 'anonymous'
      throw new Error(`No provider registered for ${name}`)
    }
    const instance = factory(this) as T
    this.instances.set(provider, instance)
    return instance
  }

  has(provider: Provider<unknown>): boolean {
    return this.factories.has(provider)
  }
}
