import { Container, describeToken, type Constructor } from './container'
import {
  type Module,
  type Provider,
  providerToken,
  isClassProvider,
} from './module'
import { Token } from './token'
import { disposeAll, hasAsyncDispose, hasInit } from './lifecycle'

export interface Context extends AsyncDisposable {
  resolve<T>(token: Token<T> | Constructor<T>): T
}

export interface ContextOptions {
  imports: Module[]
}

interface BootResult {
  container: Container
  modules: Module[]
  initOrder: unknown[]
}

export async function createContext(options: ContextOptions): Promise<Context> {
  const boot = await bootGraph(options.imports)
  let disposed = false
  return {
    resolve(provider) {
      return boot.container.resolve(provider)
    },
    async [Symbol.asyncDispose]() {
      if (disposed) return
      disposed = true
      await disposeProviders(boot)
    },
  }
}

/**
 * The shared boot pipeline used by both `createApp` and `createContext`.
 * Flatten → validate (visibility + duplicates + missing tokens) → construct
 * (topological) → init (topological). On init failure: dispose the
 * already-init'd providers in reverse order and rethrow.
 */
export async function bootGraph(imports: Module[]): Promise<BootResult> {
  const modules = flattenModules(imports)
  const declaringModuleByToken = new Map<unknown, Module>()
  const visibilityByModule = computeVisibility(modules)

  for (const mod of modules) {
    for (const provider of mod.providers ?? []) {
      const token = providerToken(provider)
      const prior = declaringModuleByToken.get(token)
      if (prior) {
        throw new Error(
          `duplicate provider token ${describeToken(token)} declared in two modules`,
        )
      }
      declaringModuleByToken.set(token, mod)
    }
  }

  const container = new Container()

  for (const mod of modules) {
    for (const provider of mod.providers ?? []) {
      container.registerProvider(provider, mod)
    }
    for (const Ctrl of mod.controllers ?? []) {
      if (!container.has(Ctrl)) container.registerProvider(Ctrl, mod)
    }
  }

  validateVisibility(modules, declaringModuleByToken, visibilityByModule)

  await container.boot()

  const initOrder = container.initOrder()
  const initialized: AsyncDisposable[] = []
  try {
    for (const inst of initOrder) {
      if (hasInit(inst)) await inst.init()
      if (hasAsyncDispose(inst)) initialized.push(inst)
    }
  } catch (e) {
    await disposeAll([...initialized].reverse()).catch(() => undefined)
    throw e
  }

  return { container, modules, initOrder }
}

export async function disposeProviders(boot: BootResult): Promise<void> {
  const disposables: AsyncDisposable[] = []
  for (const inst of boot.container.disposalOrder()) {
    if (hasAsyncDispose(inst)) disposables.push(inst)
  }
  await disposeAll(disposables)
}

export function flattenModules(roots: Module[]): Module[] {
  const seen = new Set<Module>()
  const out: Module[] = []
  const visit = (m: Module): void => {
    if (seen.has(m)) return
    seen.add(m)
    for (const child of m.imports ?? []) visit(child)
    out.push(m)
  }
  for (const m of roots) visit(m)
  return out
}

/**
 * Build, for every module M, the set of tokens M can see — its own providers
 * plus exported tokens from its direct imports (which themselves include
 * re-exports re-exported from deeper imports). A token re-exports through a
 * chain only if every hop names it in `exports`.
 */
function computeVisibility(modules: readonly Module[]): Map<Module, Set<unknown>> {
  const exportsByModule = new Map<Module, Set<unknown>>()
  for (const mod of modules) {
    exportsByModule.set(mod, new Set((mod.exports ?? []) as unknown[]))
  }

  const ownTokens = new Map<Module, Set<unknown>>()
  for (const mod of modules) {
    const own = new Set<unknown>()
    for (const p of mod.providers ?? []) own.add(providerToken(p))
    for (const Ctrl of mod.controllers ?? []) own.add(Ctrl)
    ownTokens.set(mod, own)
  }

  const visibleExportsCache = new Map<Module, Set<unknown>>()
  const visibleExportsOf = (mod: Module): Set<unknown> => {
    if (visibleExportsCache.has(mod)) return visibleExportsCache.get(mod)!
    const declared = exportsByModule.get(mod) ?? new Set<unknown>()
    const own = ownTokens.get(mod) ?? new Set<unknown>()
    const result = new Set<unknown>()
    for (const token of declared) {
      if (own.has(token)) result.add(token)
    }
    for (const child of mod.imports ?? []) {
      const childExports = visibleExportsOf(child)
      for (const token of declared) {
        if (childExports.has(token)) result.add(token)
      }
    }
    visibleExportsCache.set(mod, result)
    return result
  }

  const visible = new Map<Module, Set<unknown>>()
  for (const mod of modules) {
    const set = new Set<unknown>(ownTokens.get(mod))
    for (const child of mod.imports ?? []) {
      for (const token of visibleExportsOf(child)) set.add(token)
    }
    visible.set(mod, set)
  }
  return visible
}

function validateVisibility(
  modules: readonly Module[],
  declaringModuleByToken: Map<unknown, Module>,
  visibility: Map<Module, Set<unknown>>,
): void {
  for (const mod of modules) {
    const visible = visibility.get(mod) ?? new Set<unknown>()
    for (const provider of mod.providers ?? []) {
      checkProviderDeps(provider, visible, declaringModuleByToken)
    }
    for (const Ctrl of mod.controllers ?? []) {
      const deps = (Ctrl as Constructor & {
        inject?: ReadonlyArray<Token<any> | Constructor<any>>
      }).inject
      if (!deps) continue
      for (const dep of deps) {
        ensureVisible(dep, Ctrl, visible, declaringModuleByToken)
      }
    }
  }
}

function checkProviderDeps(
  provider: Provider,
  visible: Set<unknown>,
  declaringModuleByToken: Map<unknown, Module>,
): void {
  if (isClassProvider(provider)) {
    const deps = (provider as Constructor & {
      inject?: ReadonlyArray<Token<any> | Constructor<any>>
    }).inject
    if (!deps) return
    for (const dep of deps) ensureVisible(dep, provider, visible, declaringModuleByToken)
    return
  }
  if ('useFactory' in provider) {
    const inject = provider.inject ?? []
    for (const dep of inject)
      ensureVisible(dep, provider.provide, visible, declaringModuleByToken)
  }
}

function ensureVisible(
  dep: Token<any> | Constructor<any>,
  consumer: unknown,
  visible: Set<unknown>,
  declaringModuleByToken: Map<unknown, Module>,
): void {
  if (visible.has(dep)) return
  const declaring = declaringModuleByToken.get(dep)
  if (!declaring) {
    throw new Error(
      `missing provider for ${describeToken(dep)} (required by ${describeConsumer(consumer)})`,
    )
  }
  throw new Error(
    `visibility violation: ${describeConsumer(consumer)} requires ${describeToken(dep)} which is declared in another module but not exported`,
  )
}

function describeConsumer(c: unknown): string {
  if (c instanceof Token) return `Token(${c.name})`
  if (typeof c === 'function') return (c as { name?: string }).name || 'anonymous'
  if (c && typeof c === 'object' && 'provide' in c)
    return describeToken((c as { provide: Token<any> | Constructor<any> }).provide)
  return String(c)
}
