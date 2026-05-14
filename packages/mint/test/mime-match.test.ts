import { describe, expect, it } from 'vitest'
import { matchMimeType } from '../src/mime'

describe('matchMimeType', () => {
  it('matches exact MIME types', () => {
    expect(matchMimeType('image/png', ['image/png'])).toBe(true)
    expect(matchMimeType('image/png', ['image/jpeg', 'image/png'])).toBe(true)
  })

  it('matches subtype wildcards', () => {
    expect(matchMimeType('image/jpeg', ['image/*'])).toBe(true)
    expect(matchMimeType('image/svg+xml', ['image/*'])).toBe(true)
    expect(matchMimeType('text/html', ['text/*'])).toBe(true)
  })

  it('matches universal wildcards', () => {
    expect(matchMimeType('application/pdf', ['*/*'])).toBe(true)
    expect(matchMimeType('image/png', ['*/*'])).toBe(true)
  })

  it('rejects mismatched MIME types', () => {
    expect(matchMimeType('text/plain', ['image/*'])).toBe(false)
    expect(matchMimeType('image/png', ['image/jpeg'])).toBe(false)
    expect(matchMimeType('image/png', [])).toBe(false)
  })

  it('strips parameters from the actual type before matching', () => {
    expect(matchMimeType('text/plain; charset=utf-8', ['text/plain'])).toBe(true)
    expect(matchMimeType('image/jpeg;quality=high', ['image/jpeg'])).toBe(true)
  })

  it('is case-insensitive', () => {
    expect(matchMimeType('IMAGE/PNG', ['image/png'])).toBe(true)
    expect(matchMimeType('Image/JPEG', ['image/*'])).toBe(true)
  })

  it('rejects empty actual', () => {
    expect(matchMimeType('', ['image/*'])).toBe(false)
  })

  it('handles type-side wildcards', () => {
    expect(matchMimeType('image/png', ['*/png'])).toBe(true)
    expect(matchMimeType('image/png', ['*/jpeg'])).toBe(false)
  })
})
