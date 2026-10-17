// Shared fake for the chat reader view (chat-reader and the UI audit).
// The trip chat's last message becomes a newsletter with reader blocks; the HTML body is what Show original renders.
const paragraphs = Array.from({ length: 10 }, (_, i) => `{ kind: 'text', spans: [{ text: 'Trail note ${i + 1}.' }] }`).join(', ')
exports.readerFake = `{ const A = window.go.app.App, get = A.GetConversation
  A.GetConversation = async (id) => {
    const c = await get(id)
    if (id !== 't-trip') return c
    const last = c.messages.length - 1
    const reader = [
      { kind: 'heading', spans: [{ text: 'Ten autumn hikes' }] },
      { kind: 'image', src: 'https://cdn.example.com/hero.jpg', alt: 'A ridge at dawn' },
      { kind: 'text', spans: [{ text: 'Coastal paths and ' }, { text: 'a ridge walk', href: 'https://example.com/ridge' }, { text: ' this week.' }] },
      { kind: 'item', spans: [{ text: 'Boots' }] },
      { kind: 'text', spans: [{ text: 'Read more', href: 'https://example.com/read' }] },
      ${paragraphs},
      { kind: 'text', spans: [{ text: 'Unsubscribe', href: 'https://example.com/unsub' }] },
    ]
    c.messages[last] = { ...c.messages[last], subject: 'Trail digest', bodyHtml: '<table role="presentation"><tr><td><h1>Ten autumn hikes</h1></td></tr></table>',
      chat: { text: 'Ten autumn hikes', hasQuoted: false, isRich: true, reader } }
    return c
  } }`
