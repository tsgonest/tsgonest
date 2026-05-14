import type { Constructor } from './container'
import type { Token } from './token'
import type { Middleware } from './types'

export type Scope = 'singleton' | 'transient'

export interface ClassWithInject<T = unknown> {
  new (...args: any[]): T
  inject?: ReadonlyArray<Token<any> | Constructor<any>>
}

export interface ValueProvider<T = unknown> {
  provide: Token<T>
  useValue: T
}

export interface FactoryProvider<T = unknown> {
  provide: Token<T>
  useFactory: (...deps: any[]) => T | Promise<T>
  inject?: ReadonlyArray<Token<any> | Constructor<any>>
  scope?: Scope
}

export type Provider<T = unknown> =
  | Constructor<T>
  | ValueProvider<T>
  | FactoryProvider<T>

export interface Module {
  imports?: Module[]
  providers?: Provider[]
  exports?: Array<Token<any> | Constructor<any>>
  controllers?: Constructor[]
  middleware?: Middleware[]
}

export function defineModule(m: Module): Module {
  return m
}

/**
 * Sugar for the `useFactory` + `inject` provider form, packaged as a single-
 * provider module. Useful for dynamic module patterns like
 * `StorageModuleAsync({ inject, useFactory })`.
 */
export function defineAsyncModule<T>(opts: {
  provide: Token<T>
  inject?: ReadonlyArray<Token<any> | Constructor<any>>
  useFactory: (...deps: any[]) => T | Promise<T>
  exports?: boolean
}): Module {
  return {
    providers: [
      {
        provide: opts.provide,
        useFactory: opts.useFactory,
        inject: opts.inject,
      } satisfies FactoryProvider<T>,
    ],
    exports: opts.exports === false ? [] : [opts.provide],
  }
}

export function isClassProvider(p: Provider): p is Constructor {
  return typeof p === 'function'
}

export function isValueProvider(p: Provider): p is ValueProvider {
  return typeof p === 'object' && p !== null && 'useValue' in p
}

export function isFactoryProvider(p: Provider): p is FactoryProvider {
  return typeof p === 'object' && p !== null && 'useFactory' in p
}

export function providerToken(p: Provider): Token<any> | Constructor<any> {
  if (isClassProvider(p)) return p
  return p.provide
}
