/**
 * Mint's decorators are compile-time-only sugar. tsgonest analyses them
 * statically and erases them in the emitted output, so most exports here are
 * runtime no-ops that return the target untouched. No `reflect-metadata`
 * needed.
 *
 * Each factory accepts any args (TS 5 legacy decorators may receive 1-3
 * arguments depending on whether they decorate a class, method, property, or
 * parameter) and just hands the target back.
 *
 * `@UseMiddleware` is the one decorator that also has a runtime effect: it
 * records the middleware list against the target class (and optionally method)
 * in a `WeakMap`, so a future codegen step can walk the binding at registration
 * time. For Phase 4 the codegen does NOT yet read these maps, but recording is
 * cheap and makes user code using `@UseMiddleware` work without crashing.
 */

import type { Middleware } from './types'

type AnyDecorator = (...args: any[]) => any

type Constructor = new (...args: any[]) => unknown

const noop: AnyDecorator = (..._args: any[]) => {
  // Legacy decorators are called as `Decorator(target, key?, descriptor?)`.
  // Returning undefined leaves the original descriptor / target in place.
  return undefined
}

/** Class decorators */
export const Controller = (..._args: any[]): AnyDecorator => noop
export const Injectable = (..._args: any[]): AnyDecorator => noop

/** Method (route) decorators */
export const Get = (..._args: any[]): AnyDecorator => noop
export const Post = (..._args: any[]): AnyDecorator => noop
export const Put = (..._args: any[]): AnyDecorator => noop
export const Patch = (..._args: any[]): AnyDecorator => noop
export const Delete = (..._args: any[]): AnyDecorator => noop
export const Head = (..._args: any[]): AnyDecorator => noop
export const Options = (..._args: any[]): AnyDecorator => noop

/** Parameter decorators */
export const Body = (..._args: any[]): AnyDecorator => noop
export const Query = (..._args: any[]): AnyDecorator => noop
export const Param = (..._args: any[]): AnyDecorator => noop
export const Headers = (..._args: any[]): AnyDecorator => noop
export const Ctx = (..._args: any[]): AnyDecorator => noop
export const Inject = (..._args: any[]): AnyDecorator => noop

const classMiddleware = new WeakMap<Constructor, Middleware[]>()
const methodMiddleware = new WeakMap<Constructor, Map<string | symbol, Middleware[]>>()

/**
 * Attaches middleware to a controller class (when applied to a class) or to a
 * single method (when applied to a method). Records into module-level
 * WeakMaps; a future codegen revision will read these at boot to wire the
 * recorded middleware into the chain. For Phase 4 the recording is the only
 * effect — controller/method @UseMiddleware is not yet enforced at runtime by
 * core.
 */
export function UseMiddleware(...mws: Middleware[]): AnyDecorator {
  return (target: unknown, propertyKey?: string | symbol): unknown => {
    if (typeof target === 'function' && propertyKey === undefined) {
      const Ctor = target as Constructor
      const existing = classMiddleware.get(Ctor) ?? []
      classMiddleware.set(Ctor, [...existing, ...mws])
      return undefined
    }
    if (typeof target === 'object' && target !== null && propertyKey !== undefined) {
      const Ctor = (target as { constructor: Constructor }).constructor
      let methodMap = methodMiddleware.get(Ctor)
      if (!methodMap) {
        methodMap = new Map()
        methodMiddleware.set(Ctor, methodMap)
      }
      const existing = methodMap.get(propertyKey) ?? []
      methodMap.set(propertyKey, [...existing, ...mws])
      return undefined
    }
    return undefined
  }
}

/** Read the class-level middleware list recorded by `@UseMiddleware`. */
export function getControllerMiddleware(Ctor: Constructor): readonly Middleware[] {
  return classMiddleware.get(Ctor) ?? []
}

/** Read the method-level middleware list recorded by `@UseMiddleware`. */
export function getMethodMiddleware(
  Ctor: Constructor,
  method: string | symbol,
): readonly Middleware[] {
  return methodMiddleware.get(Ctor)?.get(method) ?? []
}
