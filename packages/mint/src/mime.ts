/**
 * Match a concrete MIME type (e.g. `image/png`) against a list of allowed
 * patterns. Patterns support a single `'*'` subtype wildcard (`image/*`) or
 * a fully-qualified wildcard (`'*&#47;*'`). Exact matches are case-insensitive
 * on the type/subtype but preserve the original value.
 *
 * @example
 *   matchMimeType('image/png', ['image/png']) // true
 *   matchMimeType('image/jpeg', ['image/*'])   // true
 *   matchMimeType('text/plain', ['image/*'])   // false
 */
export function matchMimeType(actual: string, allowed: readonly string[]): boolean {
  if (!actual) return false
  const normalized = normalizeMime(actual)
  for (const pattern of allowed) {
    if (matchOne(normalized, normalizeMime(pattern))) {
      return true
    }
  }
  return false
}

function normalizeMime(value: string): string {
  // Strip any parameter (charset, boundary, etc.) and trim/lowercase the
  // type+subtype part. RFC 6838: parameters follow `;`.
  const idx = value.indexOf(';')
  const main = (idx >= 0 ? value.slice(0, idx) : value).trim().toLowerCase()
  return main
}

function matchOne(actual: string, pattern: string): boolean {
  if (pattern === '*/*' || pattern === '*') return true
  if (pattern === actual) return true
  const slash = pattern.indexOf('/')
  if (slash < 0) return false
  const [type, subtype] = [pattern.slice(0, slash), pattern.slice(slash + 1)]
  const actualSlash = actual.indexOf('/')
  if (actualSlash < 0) return false
  const [aType, aSub] = [actual.slice(0, actualSlash), actual.slice(actualSlash + 1)]
  if (type === '*' && subtype === aSub) return true
  if (subtype === '*' && type === aType) return true
  return false
}
