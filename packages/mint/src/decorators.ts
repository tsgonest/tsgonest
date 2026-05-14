/**
 * Mint's decorators are compile-time-only sugar. tsgonest analyses them
 * statically and erases them in the emitted output, so every export here is a
 * runtime no-op that returns the target untouched. No `reflect-metadata`
 * needed.
 *
 * Each factory accepts any args (TS 5 legacy decorators may receive 1-3
 * arguments depending on whether they decorate a class, method, property, or
 * parameter) and just hands the target back.
 */

type AnyDecorator = (...args: any[]) => any

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

/** Method-level middleware attachment */
export const UseMiddleware = (..._args: any[]): AnyDecorator => noop
