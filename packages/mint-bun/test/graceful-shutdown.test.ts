import { describe, it, expect, afterEach } from 'vitest'
import { createApp, defineModule } from '@mintkit/core'
import { gracefulShutdown } from '../src/graceful-shutdown'

class Noop {}

// Track any unsubscribe functions so even a failed test doesn't leave signal
// listeners attached.
const cleanups: Array<() => void> = []
afterEach(() => {
  while (cleanups.length) cleanups.pop()!()
})

describe('gracefulShutdown', () => {
  it('calls app[Symbol.asyncDispose] when the configured signal fires', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [Noop] })],
    })
    let disposed = 0
    const original = app[Symbol.asyncDispose].bind(app)
    app[Symbol.asyncDispose] = async () => {
      disposed++
      await original()
    }

    const unsub = gracefulShutdown(app, {
      signals: ['SIGUSR2'],
      timeout: 1000,
    })
    cleanups.push(unsub)

    process.emit('SIGUSR2' as any)
    // Allow the listener's async dispose to flush.
    await new Promise<void>((r) => setImmediate(r))
    expect(disposed).toBe(1)
  })

  it('unsubscribe removes the registered listener', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [Noop] })],
    })
    let disposed = 0
    app[Symbol.asyncDispose] = async () => {
      disposed++
    }

    const before = process.listenerCount('SIGUSR2')
    const unsub = gracefulShutdown(app, {
      signals: ['SIGUSR2'],
      timeout: 1000,
    })
    expect(process.listenerCount('SIGUSR2')).toBe(before + 1)
    unsub()
    expect(process.listenerCount('SIGUSR2')).toBe(before)

    process.emit('SIGUSR2' as any)
    await new Promise<void>((r) => setImmediate(r))
    expect(disposed).toBe(0)
  })

  it('defaults to SIGTERM and SIGINT when no signals given', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [Noop] })],
    })
    const beforeTerm = process.listenerCount('SIGTERM')
    const beforeInt = process.listenerCount('SIGINT')
    const unsub = gracefulShutdown(app)
    cleanups.push(unsub)
    expect(process.listenerCount('SIGTERM')).toBe(beforeTerm + 1)
    expect(process.listenerCount('SIGINT')).toBe(beforeInt + 1)
  })

  it('hard-exits if dispose exceeds the timeout', async () => {
    const app = await createApp({
      imports: [defineModule({ controllers: [Noop] })],
    })
    // A dispose that never resolves: timeout should kick in.
    app[Symbol.asyncDispose] = () => new Promise<void>(() => {})

    const exits: number[] = []
    const realExit = process.exit
    // process.exit is typed `(code: never) => never`; we stub via Object.defineProperty
    // so TS-strict callers don't complain.
    Object.defineProperty(process, 'exit', {
      value: (code?: number) => {
        exits.push(code ?? 0)
      },
      configurable: true,
    })

    try {
      const unsub = gracefulShutdown(app, {
        signals: ['SIGUSR2'],
        timeout: 25,
      })
      cleanups.push(unsub)
      process.emit('SIGUSR2' as any)
      await new Promise<void>((r) => setTimeout(r, 75))
      expect(exits).toEqual([1])
    } finally {
      Object.defineProperty(process, 'exit', {
        value: realExit,
        configurable: true,
      })
    }
  })
})
