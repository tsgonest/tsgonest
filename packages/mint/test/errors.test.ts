import { describe, it, expect } from 'vitest'
import {
  HttpError,
  BadRequestError,
  UnauthorizedError,
  ForbiddenError,
  NotFoundError,
  ConflictError,
  UnprocessableEntityError,
  TooManyRequestsError,
  InternalServerError,
} from '../src/errors'

describe('HttpError hierarchy', () => {
  it('HttpError carries status, body, and message', () => {
    const err = new HttpError(418, { teapot: true }, "I'm a teapot")
    expect(err.status).toBe(418)
    expect(err.body).toEqual({ teapot: true })
    expect(err.message).toBe("I'm a teapot")
    expect(err.name).toBe('HttpError')
    expect(err).toBeInstanceOf(Error)
  })

  it('default message uses the status when none provided', () => {
    const err = new HttpError(500)
    expect(err.message).toBe('HTTP 500')
    expect(err.body).toBeUndefined()
  })

  it('subclasses carry the right status and name', () => {
    const cases: Array<[new (b?: unknown, m?: string) => HttpError, number, string]> = [
      [BadRequestError, 400, 'BadRequestError'],
      [UnauthorizedError, 401, 'UnauthorizedError'],
      [ForbiddenError, 403, 'ForbiddenError'],
      [NotFoundError, 404, 'NotFoundError'],
      [ConflictError, 409, 'ConflictError'],
      [UnprocessableEntityError, 422, 'UnprocessableEntityError'],
      [TooManyRequestsError, 429, 'TooManyRequestsError'],
      [InternalServerError, 500, 'InternalServerError'],
    ]
    for (const [Ctor, expectedStatus, expectedName] of cases) {
      const e = new Ctor({ msg: 'x' })
      expect(e.status).toBe(expectedStatus)
      expect(e.body).toEqual({ msg: 'x' })
      expect(e.name).toBe(expectedName)
      expect(e).toBeInstanceOf(HttpError)
      expect(e).toBeInstanceOf(Error)
    }
  })

  it('subclasses preserve a custom message', () => {
    const e = new NotFoundError({ resource: 'user' }, 'user 42 not found')
    expect(e.message).toBe('user 42 not found')
    expect(e.status).toBe(404)
  })
})
