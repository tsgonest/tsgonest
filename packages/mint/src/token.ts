/**
 * A typed dependency-injection token. The `_type` field is a phantom — it only
 * exists at the type level and carries `T` so consumers of {@link Container.resolve}
 * get the correct return type.
 */
export class Token<T> {
  /** Phantom: never read at runtime, only used to carry the type parameter. */
  declare readonly _type: T

  constructor(public readonly name: string) {}
}

export function defineToken<T>(name: string): Token<T> {
  return new Token<T>(name)
}
