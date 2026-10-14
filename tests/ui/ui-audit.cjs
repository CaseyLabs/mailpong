// Deterministic UI quality audit for one rendered configuration. Each finding is
// { rule, element, detail }: `element` is a short, stable descriptor (tag, id, classes, label, landmark),
// never pixel coordinates. Keyboard (focus-*) findings also carry `landmark`, the descriptor of the closest
// landmark or dialog around the focused element. Allowlist entries (ui-audit-allow.cjs) document intentional behavior.
//
// Rules:
//   page-overflow     the document itself scrolls (the app is a fixed, full-window layout)
//   h-scroll          a pane scrolls horizontally (only vertical scrolling is intended)
//   clipped-text      text cut off by an overflow:hidden/clip ancestor without ellipsis or line clamp
//   offscreen-text    text outside the window and not inside a scroll container
//   clipped-control   a control partly cut off by a non-scrolling ancestor or the window
//   covered-control   another element sits on top of a visible control's centre
//   hidden-focusable  a keyboard-focusable control inside aria-hidden content
//   no-name           an interactive accessibility node without an accessible name
//   contrast          text below 4.5:1 (3:1 for large text) against its resolved background
//   focus-lost        Tab moved focus to <body> or nowhere
//   focus-trap        Tab stopped advancing
//   focus-invisible   the focused element is not visible
//   focus-indicator   focusing an element changes nothing visible on it or its close ancestors/children
//   focus-unchecked   a focused element had no unfocused baseline, so its focus indicator could not be checked

// Browser-side helpers, installed once per page as window.__audit.
const browserHelpers = () => {
  const interactive = 'a[href],button,input:not([type=hidden]),select,textarea,summary,[role=button],[role=link],[role=checkbox],[role=switch],[role=tab],[role=menuitem],[role=option],[role=radio],[role=combobox],[role=slider],[role=textbox],[tabindex]:not([tabindex="-1"]),[contenteditable="true"]'
  const overlay = '[role=dialog],[role=alertdialog],[role=menu],[role=listbox],[role=tooltip],[popover],[data-radix-popper-content-wrapper],[data-bits-floating-content]'
  const clean = (s) => (s ?? '').replace(/\s+/g, ' ').trim()
  const short = (s, n = 40) => (s.length > n ? s.slice(0, n - 1) + '…' : s)
  const one = (el) => {
    let s = el.tagName.toLowerCase()
    if (el.id) s += '#' + el.id
    const cls = [...el.classList].filter((c) => /^[a-z][\w-]*$/i.test(c) && !/^(flex|grid|items|justify|gap|p[xytrbl]?|m[xytrbl]?|w|h|min|max|text|bg|border|rounded|shrink|grow|overflow|relative|absolute|inline|block|hidden|truncate|cursor|transition|duration|select|z|top|left|right|bottom|leading|font|tracking|opacity|shadow|outline|ring)(-|$)/.test(c))
    if (cls.length) s += '.' + cls.slice(0, 3).join('.')
    const label = el.getAttribute('aria-label') || el.getAttribute('title') || el.getAttribute('placeholder')
    if (label) s += `[${short(clean(label))}]`
    return s
  }
  // "button.chat-row[Priya Shah] "Weekend trip…" in nav[Accounts]": the element plus its nearest named region.
  const describe = (el) => {
    if (!el || el.nodeType !== 1) return String(el)
    let s = one(el)
    const text = clean(el.innerText)
    if (!el.getAttribute('aria-label') && text) s += ` "${short(text, 30)}"`
    // An unlabelled, textless element (an icon button missing its name) is located by a nearby tooltip.
    if (!text && !el.getAttribute('aria-label') && !el.getAttribute('title')) {
      let a = el.parentElement
      for (let i = 0; a && i < 3 && !a.getAttribute('title'); i++) a = a.parentElement
      if (a?.getAttribute('title')) s += ` near[${short(clean(a.getAttribute('title')))}]`
    }
    const region = el.parentElement?.closest('[aria-label],[role=dialog],[role=navigation],[role=main],[role=region],nav,main,aside,header,[id]')
    if (region) s += ' in ' + one(region)
    return s
  }
  const visible = (el) => el.checkVisibility({ opacityProperty: true, visibilityProperty: true }) && el.getClientRects().length > 0
  const scrolls = (v) => v === 'auto' || v === 'scroll'
  const clips = (v) => v === 'hidden' || v === 'clip'
  const srOnly = (el) => {
    for (let e = el; e && e !== document.body; e = e.parentElement) {
      const r = e.getBoundingClientRect(), cs = getComputedStyle(e)
      if ((r.width <= 1 && r.height <= 1 && clips(cs.overflowX)) || /rect\(0/.test(cs.clip) || /inset\(50%\)/.test(cs.clipPath)) return true
    }
    return false
  }
  // Visible box of el after every clipping ancestor and the window; `scrolled` when a scroll container clipped it.
  const visibleBox = (el, rect = el.getBoundingClientRect()) => {
    let { left, top, right, bottom } = rect, scrolled = false, clipper = null
    for (let a = el.parentElement; a && a !== document.documentElement; a = a.parentElement) {
      const cs = getComputedStyle(a)
      const sx = scrolls(cs.overflowX), sy = scrolls(cs.overflowY), cx = clips(cs.overflowX), cy = clips(cs.overflowY)
      if (!(sx || sy || cx || cy)) continue
      const r = a.getBoundingClientRect()
      const before = [left, top, right, bottom]
      if (sx || cx) { left = Math.max(left, r.left); right = Math.min(right, r.right) }
      if (sy || cy) { top = Math.max(top, r.top); bottom = Math.min(bottom, r.bottom) }
      const changed = before[0] !== left || before[1] !== top || before[2] !== right || before[3] !== bottom
      if (changed && (sx || sy)) scrolled = true
      if (changed && !(sx || sy) && !clipper) clipper = a
    }
    const vw = document.documentElement.clientWidth, vh = document.documentElement.clientHeight
    const window = left < 0 || top < 0 || right > vw || bottom > vh
    left = Math.max(left, 0); top = Math.max(top, 0); right = Math.min(right, vw); bottom = Math.min(bottom, vh)
    return { left, top, right, bottom, width: Math.max(0, right - left), height: Math.max(0, bottom - top), scrolled, clipper, window }
  }
  const ellipsized = (from, to) => {
    for (let e = from; e; e = e.parentElement) {
      const cs = getComputedStyle(e)
      if (cs.textOverflow === 'ellipsis' || (cs.webkitLineClamp && cs.webkitLineClamp !== 'none')) return true
      if (e === to) return false
    }
    return false
  }
  // Canvas normalizes any CSS colour (oklch, color-mix, named) to sRGB.
  const ctx = Object.assign(document.createElement('canvas'), { width: 1, height: 1 }).getContext('2d', { willReadFrequently: true })
  const rgba = (c) => {
    ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = '#000'; ctx.fillStyle = c; ctx.fillRect(0, 0, 1, 1)
    const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data
    return [r, g, b, a / 255]
  }
  const over = ([r, g, b, a], [R, G, B]) => [r * a + R * (1 - a), g * a + G * (1 - a), b * a + B * (1 - a), 1]
  const lum = ([r, g, b]) => {
    const f = (v) => { v /= 255; return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4 }
    return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b)
  }
  const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05) }
  // Background behind el: composite ancestor backgrounds until an opaque one; null under images/gradients.
  const background = (el) => {
    const layers = []
    for (let e = el; e; e = e.parentElement) {
      const cs = getComputedStyle(e)
      if (cs.backgroundImage !== 'none') return null
      const c = rgba(cs.backgroundColor)
      if (c[3] > 0) layers.push(c)
      if (c[3] >= 1) break
    }
    return layers.reverse().reduce((base, layer) => over(layer, base), [255, 255, 255, 1])
  }
  // A fixed or absolute layer covering at least half the window is an intentional overlay (responsive viewer, scrim).
  const layer = (el) => {
    const area = document.documentElement.clientWidth * document.documentElement.clientHeight
    for (let e = el; e && e !== document.body; e = e.parentElement) {
      const cs = getComputedStyle(e)
      if (cs.position !== 'fixed' && cs.position !== 'absolute') continue
      const r = e.getBoundingClientRect()
      if (r.width * r.height >= area / 2) return e
    }
    return null
  }
  const opacity = (el) => { let o = 1; for (let e = el; e; e = e.parentElement) o *= Number(getComputedStyle(e).opacity); return o }

  const pageChecks = () => {
    const out = []
    const add = (rule, el, detail) => out.push({ rule, element: typeof el === 'string' ? el : describe(el), detail })
    const de = document.documentElement
    if (de.scrollWidth > de.clientWidth + 1) add('page-overflow', 'html', `document is ${de.scrollWidth}px wide in a ${de.clientWidth}px window`)
    if (de.scrollHeight > de.clientHeight + 1) add('page-overflow', 'html', `document is ${de.scrollHeight}px tall in a ${de.clientHeight}px window`)
    const modal = [...document.querySelectorAll('[role=dialog][aria-modal=true],[role=alertdialog]')].find(visible)

    for (const el of document.querySelectorAll('body *')) {
      const cs = getComputedStyle(el)
      if (scrolls(cs.overflowX) && el.scrollWidth > el.clientWidth + 1 && visible(el)) {
        add('h-scroll', el, `content is ${el.scrollWidth}px wide in a ${el.clientWidth}px pane`)
      }
    }

    // Text: clipping, off-window placement and contrast, per text node.
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
    const seen = new Set()
    for (let n = walker.nextNode(); n; n = walker.nextNode()) {
      const el = n.parentElement
      if (!el || !/[\p{L}\p{N}]/u.test(n.data) || !visible(el) || el.closest('script,style,noscript,[aria-hidden=true] svg')) continue
      const range = document.createRange(); range.selectNodeContents(n)
      const rect = range.getBoundingClientRect()
      if (rect.width < 1 || rect.height < 1 || srOnly(el)) continue
      const box = visibleBox(el, rect)
      if (box.clipper && !ellipsized(el, box.clipper) && (box.width < rect.width - 1 || box.height < rect.height - 1) && box.width > 0 && box.height > 0) {
        add('clipped-text', el, `text "${short(clean(n.data), 30)}" is cut off by ${describe(box.clipper)}`)
      }
      if (box.window && !box.scrolled && !box.clipper && box.width < rect.width - 1) add('offscreen-text', el, `text "${short(clean(n.data), 30)}" extends outside the window`)
      if (box.width < 1 || box.height < 1) continue
      // Contrast only for text that is actually on top at its centre.
      const cx = box.left + box.width / 2, cy = box.top + box.height / 2
      const top = document.elementFromPoint(cx, cy)
      if (!top || !(el === top || el.contains(top) || top.contains(el))) continue
      if (el.closest('[disabled],[aria-disabled=true]') || seen.has(el)) continue
      seen.add(el)
      const cs = getComputedStyle(el)
      const bg = background(el)
      if (!bg) continue
      const fg = rgba(cs.color); fg[3] *= opacity(el)
      if (fg[3] === 0) continue
      const size = parseFloat(cs.fontSize), large = size >= 24 || (size >= 18.66 && Number(cs.fontWeight) >= 700)
      const r = ratio(over(fg, bg), bg), min = large ? 3 : 4.5
      if (r < min - 0.005) add('contrast', el, `${r.toFixed(2)}:1 < ${min}:1 (text ${cs.color} on rgb(${bg.slice(0, 3).map(Math.round).join(' ')}))`)
    }

    for (const el of document.querySelectorAll(interactive)) {
      if (el.matches(':disabled') || !visible(el)) continue
      const hiddenBy = el.closest('[aria-hidden=true]')
      if (hiddenBy && el.tabIndex >= 0 && !el.closest('[inert]')) add('hidden-focusable', el, `focusable inside aria-hidden ${describe(hiddenBy)}`)
      if (srOnly(el)) continue
      const rect = el.getBoundingClientRect()
      if (rect.width < 1 || rect.height < 1) continue
      const box = visibleBox(el, rect)
      if (box.width === 0 || box.height === 0) {
        if (!box.scrolled && !box.clipper && box.window) add('clipped-control', el, 'control is entirely outside the window')
        continue
      }
      const partial = box.width < rect.width - 2 || box.height < rect.height - 2
      if (partial && !box.scrolled) add('clipped-control', el, box.clipper ? `cut off by ${describe(box.clipper)}` : 'cut off by the window edge')
      if (partial) continue
      const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2)
      if (!hit || el === hit || el.contains(hit) || hit.contains(el)) continue
      if ((hit.closest('label') && hit.closest('label').control === el) || hit.closest('[role=tooltip]')) continue
      if (modal && !modal.contains(el)) continue
      if (hit.closest(overlay) && !el.closest(overlay)) continue
      // Below a large positioned layer (scrim, responsive viewer, drawer) controls are intentionally covered.
      const stack = document.elementsFromPoint(box.left + box.width / 2, box.top + box.height / 2)
      const below = stack.findIndex((e) => e === el || e.contains(el))
      if (stack.slice(0, below < 0 ? stack.length : below).some((e) => layer(e) && !layer(e).contains(el))) continue
      add('covered-control', el, `covered by ${describe(hit)}`)
    }
    return out
  }

  // Signature of everything a focus indicator could change on el, its two closest ancestors and its children.
  const focusSignature = (el) => {
    const props = ['outlineStyle', 'outlineWidth', 'outlineColor', 'boxShadow', 'backgroundColor', 'borderTopColor', 'borderBottomColor', 'borderLeftColor', 'color', 'textDecorationLine', 'opacity']
    const sig = (e) => { const cs = getComputedStyle(e); return props.map((p) => cs[p]).join('|') }
    const parts = [el, el.parentElement, el.parentElement?.parentElement, ...[...el.children].slice(0, 6)]
    return parts.filter(Boolean).map(sig).join('#') + (getComputedStyle(el, '::after').content + getComputedStyle(el, '::before').content)
  }
  // Unfocused signatures of interactive elements, keyed by a data attribute. Run with nothing hovered or
  // focused, and again after each Tab step for controls rendered since (menus, lazy panes) or skipped by a
  // roving tabindex. An element whose signature includes the focused element's styling (it, a sibling, or a
  // :focus-within ancestor) waits for a later step. Returns the number of Tab-focusable elements.
  let nextId = 0
  const markFocusables = (reset) => {
    if (reset) for (const el of document.querySelectorAll('[data-audit-id]')) { delete el.dataset.auditId; delete el.__auditBlur }
    const active = document.activeElement
    let total = 0
    // [tabindex] adds elements a roving tabindex may make focusable later (tabindex="-1" for now).
    for (const el of document.querySelectorAll(`${interactive},[tabindex]`)) {
      if (el.matches(':disabled')) continue
      if (el.tabIndex >= 0) total++
      if (el.__auditBlur !== undefined) continue
      const scope = el.parentElement?.parentElement ?? el.parentElement ?? el
      if (active && active !== document.body && scope.contains(active)) continue
      el.dataset.auditId = String(nextId++)
      el.__auditBlur = focusSignature(el)
    }
    return total
  }
  const focused = () => {
    let el = document.activeElement
    while (el?.shadowRoot?.activeElement) el = el.shadowRoot.activeElement
    if (!el || el === document.body || el === document.documentElement) return { lost: true }
    const rect = el.getBoundingClientRect()
    const box = visibleBox(el, rect)
    const proxy = srOnly(el) ? (el.labels?.[0] ?? el.closest('label') ?? el.parentElement) : null
    const shown = proxy ? visible(proxy) : visible(el) && box.width > 0 && box.height > 0
    const before = el.__auditBlur
    const changed = before === undefined ? null : Boolean(proxy) || focusSignature(el) !== before
    const region = el.closest('aside,nav,main,header,footer,[role=dialog],[role=alertdialog],[role=navigation],[role=main],[role=region],[role=complementary]')
    return { key: el.dataset.auditId ?? describe(el), element: describe(el), landmark: region ? one(region) : 'body', shown, changed, iframe: el.tagName === 'IFRAME' }
  }
  return { describe, visible, pageChecks, markFocusables, focused }
}

// Accessible names from Chromium's own accessibility tree (CDP), so name computation is the browser's.
const NAMED_ROLES = new Set(['button', 'link', 'textbox', 'searchbox', 'checkbox', 'radio', 'switch', 'combobox', 'listbox', 'menuitem', 'menuitemcheckbox', 'menuitemradio', 'tab', 'slider', 'spinbutton'])
const nameChecks = async (page) => {
  const cdp = await page.context().newCDPSession(page)
  try {
    await cdp.send('Accessibility.enable')
    const { nodes } = await cdp.send('Accessibility.getFullAXTree')
    const out = []
    for (const n of nodes) {
      if (n.ignored || !NAMED_ROLES.has(n.role?.value) || (n.name?.value ?? '').trim() || !n.backendDOMNodeId) continue
      const { object } = await cdp.send('DOM.resolveNode', { backendNodeId: n.backendDOMNodeId }).catch(() => ({}))
      if (!object?.objectId) continue
      const { result } = await cdp.send('Runtime.callFunctionOn', {
        objectId: object.objectId,
        functionDeclaration: 'function () { return window.__audit.visible(this) ? window.__audit.describe(this) : null }',
        returnByValue: true,
      })
      if (result.value) out.push({ rule: 'no-name', element: result.value, detail: `${n.role.value} has no accessible name` })
    }
    return out
  } finally {
    await cdp.detach().catch(() => {})
  }
}

// Wait for running transitions and animations (theme fades, panel slides) so colors and boxes are final.
const settle = (page) => page.evaluate(() => Promise.race([
  Promise.all(document.getAnimations().filter((a) => a.effect?.getTiming().iterations !== Infinity).map((a) => a.finished.catch(() => {}))),
  new Promise((resolve) => setTimeout(resolve, 3000)),
]).then(() => new Promise(requestAnimationFrame)))

// Tab through the page from the first control; check focus never vanishes, stalls, hides or shows no indicator.
const keyboardChecks = async (page, { maxSteps = 120 } = {}) => {
  const out = []
  // Baselines need the unhovered, unfocused state, so clear both first.
  await page.mouse.move(0, 0)
  await page.evaluate(() => document.activeElement?.blur?.())
  await settle(page)
  const total = await page.evaluate(() => window.__audit.markFocusables(true))
  const seen = new Map()
  let previous = null, repeats = 0, steps = 0
  for (; steps < Math.min(maxSteps, total + 5); steps++) {
    await page.keyboard.press('Tab')
    await settle(page)
    const f = await page.evaluate(() => window.__audit.focused())
    if (f.lost) {
      // Leaving the last control returns to the document once per cycle; twice in a row means focus is lost.
      if (previous === 'lost') { out.push({ rule: 'focus-lost', element: 'body', detail: `Tab moved focus to the document after step ${steps}` }); break }
      previous = 'lost'
      continue
    }
    if (f.iframe) break
    if (f.key === previous) {
      if (++repeats >= 2) { out.push({ rule: 'focus-trap', element: f.element, detail: 'Tab does not move focus away' }); break }
      continue
    }
    repeats = 0
    if (seen.has(f.key)) break // full cycle
    seen.set(f.key, f)
    previous = f.key
    if (!f.shown) out.push({ rule: 'focus-invisible', element: f.element, landmark: f.landmark, detail: 'focused element is not visible' })
    else if (f.changed === null) out.push({ rule: 'focus-unchecked', element: f.element, landmark: f.landmark, detail: 'no unfocused baseline to compare the focus indicator with' })
    else if (!f.changed) out.push({ rule: 'focus-indicator', element: f.element, landmark: f.landmark, detail: 'no visible change when focused' })
    await page.evaluate(() => window.__audit.markFocusables())
  }
  return { findings: out, reached: seen.size, focusable: total }
}

exports.install = (page) => page.evaluate(`window.__audit = (${browserHelpers})()`)

// Run every check on the current page state. Returns { findings, reached, focusable }.
exports.audit = async (page, { keyboard = true } = {}) => {
  await settle(page)
  await exports.install(page)
  const findings = [...await page.evaluate(() => window.__audit.pageChecks()), ...await nameChecks(page)]
  if (!keyboard) return { findings }
  const k = await keyboardChecks(page)
  return { findings: [...findings, ...k.findings], reached: k.reached, focusable: k.focusable }
}

// Split findings into unexpected ones and those matched by an allowlist entry (rule + element regex + optional
// landmark regex for keyboard findings + optional config filter).
exports.filter = (findings, allow, config) => {
  const used = new Set()
  const unexpected = findings.filter((f) => {
    const entry = allow.find((a) => a.rule === f.rule && a.element.test(f.element)
      && (!a.landmark || (f.landmark !== undefined && a.landmark.test(f.landmark))) && (!a.when || a.when(config)))
    if (entry) used.add(entry)
    return !entry
  })
  return { unexpected, used }
}

// One finding per line, duplicates collapsed with a count.
exports.format = (findings, label) => {
  const counts = new Map()
  for (const f of findings) {
    const line = `${f.rule}: ${f.element} — ${f.detail}`
    counts.set(line, (counts.get(line) ?? 0) + 1)
  }
  return `${findings.length} UI audit finding(s) [${label}]:\n` + [...counts].map(([l, n]) => `  ${l}${n > 1 ? ` (×${n})` : ''}`).join('\n')
}
