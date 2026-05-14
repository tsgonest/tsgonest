import type { Constructor } from './container'
import type { Token } from './token'
import type { Middleware } from './types'

export interface ProviderEntry<T = unknown> {
  provide: Token<T> | Constructor<T>
  useFactory?: (...args: any[]) => T
  useValue?: T
  useClass?: Constructor<T>
}

export interface Module {
  imports?: Module[]
  providers?: Array<Constructor | ProviderEntry>
  exports?: Array<Constructor | Token<unknown>>
  controllers?: Constructor[]
  middleware?: Middleware[]
}

export function defineModule(m: Module): Module {
  return m
}
