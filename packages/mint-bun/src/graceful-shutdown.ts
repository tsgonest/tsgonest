import type { App } from '@mintkit/core'

export interface GracefulShutdownOptions {
  /** Signals to listen on. Default: `['SIGTERM', 'SIGINT']`. */
  signals?: NodeJS.Signals[]
  /** How long to wait for `app[Symbol.asyncDispose]()` before hard-exiting. Default: 30000ms. */
  timeout?: number
}

/**
 * Wires SIGTERM/SIGINT (configurable) to `app[Symbol.asyncDispose]()`. If
 * dispose runs past the timeout, the process hard-exits with code 1 so
 * orchestrators don't hang on a stuck drain.
 *
 * Returns an `unsubscribe` function that detaches the signal listeners. Tests
 * and long-lived embedders should always call it on teardown — otherwise the
 * listener leaks across the next reload.
 */
export function gracefulShutdown(
  app: App,
  opts: GracefulShutdownOptions = {},
): () => void {
  const signals: NodeJS.Signals[] = opts.signals ?? ['SIGTERM', 'SIGINT']
  const timeoutMs = opts.timeout ?? 30_000

  let running = false
  const handler = (): void => {
    if (running) return
    running = true

    const timer = setTimeout(() => {
      // Drain exceeded the budget — orchestrators (k8s, systemd, …) need a
      // hard cap so they can move on. Exit 1 to signal abnormal shutdown.
      process.exit(1)
    }, timeoutMs)
    // unref so the timer doesn't keep the loop alive on its own if dispose
    // somehow resolves with nothing else pending.
    if (typeof timer.unref === 'function') timer.unref()

    Promise.resolve()
      .then(() => app[Symbol.asyncDispose]())
      .catch(() => {
        // Swallow — we still want to exit cleanly even if dispose threw.
      })
      .finally(() => {
        clearTimeout(timer)
      })
  }

  for (const sig of signals) process.on(sig, handler)

  return () => {
    for (const sig of signals) process.off(sig, handler)
  }
}
