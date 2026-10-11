// Main-thread client for the spellcheck worker. Owns a single lazily-created
// Worker, maps app locales to dictionaries, and exposes promise-based check /
// suggest calls. The worker is only created once at least one spellcheckable
// language is enabled, and torn down when none are — so a user who never
// composes (or writes only CJK) pays nothing.
import { appLocaleToDict } from './locales'
import { logger } from '$lib/logger'

type Resolver = (value: { misspelled?: string[]; suggestions?: string[] }) => void

class SpellcheckClient {
  private worker: Worker | null = null
  private pending = new Map<number, Resolver>()
  private seq = 0
  private ready = false
  private readyCbs = new Set<() => void>()
  private lastReadyLog = ''

  private ensureWorker(): Worker {
    if (!this.worker) {
      // Module worker: required so `import` inside worker.ts works under the
      // Vite dev server (unbundled ESM). Vite builds it as an ES worker too
      // (see worker.format in vite.config).
      this.worker = new Worker(new URL('./worker.ts', import.meta.url), { type: 'module' })
      this.worker.onmessage = (e: MessageEvent) => this.onMessage(e.data)
      // Field-debug signals: fire only when spellcheck actually breaks.
      this.worker.onerror = (e) => logger.error(`spellcheck: worker failed to load: ${e.message || 'unknown'}`)
      this.worker.onmessageerror = () => logger.error('spellcheck: worker message error')
    }
    return this.worker
  }

  private onMessage(msg: { type: string; id?: number; misspelled?: string[]; suggestions?: string[]; locales?: string[]; loaded?: string[]; selfTest?: boolean; message?: string }): void {
    if (msg.type === 'error') {
      logger.error(`spellcheck: dictionary load failed: ${msg.message}`)
      return
    }
    if (msg.type === 'ready') {
      this.ready = true
      // One line per distinct language set — confirms init without spamming
      // on every composer open.
      const sig = `${msg.loaded?.join(',')}|${msg.selfTest}`
      if (sig !== this.lastReadyLog) {
        this.lastReadyLog = sig
        logger.info(`spellcheck ready: dictionaries=[${msg.loaded?.join(',')}] selfCheck=${msg.selfTest}`)
      }
      for (const cb of [...this.readyCbs]) cb()
      return
    }
    if (msg.id === undefined) return
    const resolve = this.pending.get(msg.id)
    if (resolve) {
      this.pending.delete(msg.id)
      resolve(msg)
    }
  }

  // Sets the active languages from app locales (svelte-i18n codes). Locales
  // without a shipped dictionary are dropped; if none remain, the worker
  // is torn down and spellcheck goes idle.
  setLanguages(appLocales: string[]): void {
    const dicts = [...new Set(appLocales.map(appLocaleToDict).filter((d): d is string => d !== null))]
    if (dicts.length === 0) {
      this.destroy()
      return
    }
    this.ready = false
    this.ensureWorker().postMessage({ type: 'setLanguages', locales: dicts })
  }

  get isActive(): boolean {
    return this.worker !== null
  }

  onReady(cb: () => void): () => void {
    if (this.ready) cb()
    this.readyCbs.add(cb)
    return () => this.readyCbs.delete(cb)
  }

  async check(words: string[]): Promise<Set<string>> {
    if (!this.worker || words.length === 0) return new Set()
    const id = ++this.seq
    const res = await new Promise<{ misspelled?: string[] }>((resolve) => {
      this.pending.set(id, resolve)
      this.worker!.postMessage({ type: 'check', id, words })
    })
    return new Set(res.misspelled ?? [])
  }

  async suggest(word: string): Promise<string[]> {
    if (!this.worker) return []
    const id = ++this.seq
    const res = await new Promise<{ suggestions?: string[] }>((resolve) => {
      this.pending.set(id, resolve)
      this.worker!.postMessage({ type: 'suggest', id, word })
    })
    return res.suggestions ?? []
  }

  addWord(word: string): void {
    this.worker?.postMessage({ type: 'addWord', word })
  }

  removeWord(word: string): void {
    this.worker?.postMessage({ type: 'removeWord', word })
  }

  destroy(): void {
    this.worker?.terminate()
    this.worker = null
    this.ready = false
    this.pending.clear()
  }
}

export const spellcheck = new SpellcheckClient()
