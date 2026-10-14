// Fake composer and send bindings for headless checks; see "Faking the composer" in SKILL.md.
// Two uses:
// - Main window: after window.__installFakes(w) from screenshot-fakes.js, call
//   window.__installComposerFakes(w). Load it the same way (fetch from :8765 and eval).
// - Detached composer: page.addInitScript({ path: '<this file>' }) before going to
//   /composer.html. It installs its own window.go and window.runtime, since that page
//   gets no bindings in a browser.
// Every call is logged to w.__log ('saveDraft', 'queue send-1', 'fire send-1', 'undo send-1 true',
// 'SEND <subject>', 'call <Binding>'). Queued sends fire after w.__sendDelay ms (default 10000)
// and report through the send:finished event, as app/send_queue.go does.
;(() => {
  const me = { id: 'id-acc1', accountId: 'acc1', email: 'alex@example.com', name: 'Alex Morgan', isDefault: true, signatureEnabled: false, signatureForNew: false }
  const account = { id: 'acc1', name: 'Personal', email: me.email, enabled: true, noOutgoingServer: false }

  // Bindings shared by both windows. emit sends a Wails event to the page.
  const composerBindings = (w, emit) => {
    w.__log = w.__log ?? []
    w.__sendDelay = w.__sendDelay ?? 10000
    const log = (s) => w.__log.push(s)
    const timers = {}
    let n = 0
    const queue = (label, message) => {
      const id = 'send-' + ++n
      log(`queue ${id} ${label}`)
      log('review token ' + (message?.review_attempt_id || message?.reviewAttemptId || ''))
      w.__lastSentMessage = message
      const result = w.__sendResult || {}
      timers[id] = setTimeout(() => {
        delete timers[id]
        log('fire ' + id)
        emit('send:finished', { id, outcome: 'accepted', warnings: '', error: '', draftId: '', ...result })
      }, w.__sendDelay)
      return id
    }
    const evidence = async (email) => { if (w.__senderEvidenceError) throw Error(w.__senderEvidenceError); return w.__senderEvidence ?? { email, state: 'configured', observedState: 'configured', source: 'local account configuration', route: 'smtp://smtp.example.com:587 (starttls)', observedAt: '', action: 'allow', guidance: 'Locally configured sender; provider enforces permission.', reviewToken: '', sendAs: 'unknown', sendOnBehalf: 'unknown', sharedRead: 'unknown' } }
    return {
      GetSenderEvidence: async (acc, email) => evidence(email),
      // The fake chat reply always goes out from the sender below.
      GetChatSenderEvidence: async () => evidence('alex@example.com'),
      GetSubmissionAttempt: async id => ({ id, draftId: 'chat-draft-1' }),
      GetSubmissionReview: async () => w.__submissionReview ?? null,
      GetChatSubmissionReview: async () => w.__submissionReview ?? null,
      // Must be {draft: {id}}: without an ID the main-window composer sends at once, with no undo.
      SaveDraft: async () => { log('saveDraft'); return { draft: { id: 'draft-1', syncStatus: 'pending' } } },
      DeleteDraft: async (id) => { log('deleteDraft ' + id) },
      SaveChatDraft: async () => { log('saveChatDraft'); return 'chat-draft-1' },
      DeleteChatDraft: async () => { log('deleteChatDraft') },
      // Mirrors the backend gate: a review needs the current token.
      QueueChatReply: async (r) => {
        const e = await evidence('alex@example.com')
        if (e.action === 'review' && r?.senderReviewToken !== e.reviewToken) throw Error('sender review required')
        return queue(JSON.stringify(r?.text ?? ''), r)
      },
      QueueMessage: async (acc, msg, draftId) => queue(`${draftId} ${msg?.subject ?? ''}`, msg),
      UndoSend: async (id) => {
        const t = timers[id]
        log(`undo ${id} ${!!t}`)
        if (!t) return false
        clearTimeout(t)
        delete timers[id]
        return true
      },
      SendMessage: async (acc, msg) => { w.__lastSentMessage = msg; log('SEND ' + (msg?.subject ?? '')); log('review token ' + (msg?.review_attempt_id || '')) },
      OpenComposerWindow: async (...a) => { log('openComposerWindow ' + JSON.stringify(a)) },
      CloseWindow: async () => { log('closeWindow') },
    }
  }

  // Main window: layer onto screenshot-fakes.js, which has blocked every write binding.
  window.__installComposerFakes = (w) => {
    const A = w.go.app.App
    Object.assign(A, composerBindings(w, (name, data) => w.runtime.EventsEmit(name, data)), {
      // From stays empty without this.
      GetAllAccountIdentities: async () => Promise.all((await A.GetAccounts()).map(async (a) => ({ account: a, identities: await A.GetIdentities(a.id) }))),
    })
    return 'ok'
  }

  if (!location.pathname.endsWith('/composer.html')) return

  // Detached composer: no Wails runtime here, so fake window.go and window.runtime.
  // Unknown bindings resolve to null; non-reads are logged as 'call <Binding>'.
  const handlers = {}
  const runtime = {
    EventsOnMultiple: (name, cb) => { (handlers[name] ??= []).push(cb); return () => { handlers[name] = handlers[name].filter((x) => x !== cb) } },
    EventsEmit: (name, ...data) => (handlers[name] ?? []).forEach((cb) => cb(...data)),
  }
  runtime.EventsOn = (name, cb) => runtime.EventsOnMultiple(name, cb, -1)
  window.runtime = new Proxy(runtime, { get: (t, k) => t[k] ?? (() => () => {}) })
  const known = {
    ...composerBindings(window, runtime.EventsEmit),
    GetComposeMode: async () => ({ accountId: 'acc1', mode: 'new', messageId: '', draftId: '' }),
    GetIdentities: async () => [me],
    GetAllAccountIdentities: async () => [{ account, identities: [me] }],
    GetAccount: async () => account,
    GetThemeMode: async () => 'dark',
    GetSystemTheme: async () => 'dark',
    SearchContacts: async () => [],
    CheckRecipientCerts: async () => ({}),
    CheckRecipientPGPKeys: async () => ({}),
  }
  const fake = new Proxy(known, {
    get: (t, k) => t[k] ?? (async () => {
      if (!/^(Get|Has|Is|Search|Check|Refresh|Notify|Lookup)/.test(String(k))) window.__log.push('call ' + String(k))
      return null
    }),
  })
  // The page's own bindings script assigns window.go; keep our fake in place of both apps.
  let go
  Object.defineProperty(window, 'go', { configurable: true, get: () => go, set: (v) => { go = v; v.app = v.app || {}; v.app.ComposerApp = fake; v.app.App = fake } })
  window.go = { app: {} }
})()
