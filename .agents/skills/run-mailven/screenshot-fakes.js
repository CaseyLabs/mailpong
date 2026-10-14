// Fake bindings for the README screenshot (docs/ss.png); see "Retaking the README screenshot" in SKILL.md.
// Call window.__installFakes(iframe.contentWindow) before the app mounts. Blocks every non-read binding.
window.__installFakes = (win) => {
  const A = win.go.app.App
  for (const k of Object.keys(A)) if (!/^(Get|List|Search|Is|Has|Find)/.test(k.replace(/^[A-Za-z]+_/, ''))) A[k] = () => Promise.reject(new Error('blocked: ' + k))
  const now = Date.now(), H = 3600e3, D = 24 * H
  const iso = (ms) => new Date(now - ms).toISOString()
  const me = { name: 'Alex Morgan', email: 'alex@example.com' }
  const acct = (id, name, email, color, order) => ({ id, name, email, color, orderIndex: order, enabled: true, imapHost: 'imap.example.com', imapPort: 993, imapSecurity: 'tls', imapAuthMechanism: 'plain', smtpHost: 'smtp.example.com', smtpPort: 465, smtpSecurity: 'tls', smtpAuthMechanism: 'plain', noOutgoingServer: false, smtpUsername: email, replyForwardIdentityId: '', authType: 'password', username: email, syncPeriodDays: 90, syncInterval: 5, syncAllFolders: false, syncFoldersEnabled: true, readReceiptRequestPolicy: 'never', createdAt: iso(300 * D), updatedAt: iso(D) })
  const accounts = [acct('acc1', 'Personal', me.email, '#88c0d0', 0), acct('acc2', 'Work', 'alex.morgan@northwind.example', '#a3be8c', 1)]
  const folder = (acc, id, name, type, unread, total) => ({ folder: { id, accountId: acc, name, path: name, type, uidValidity: 1, uidNext: 100, highestModSeq: 0, totalCount: total, unreadCount: unread, subscribed: true, lastSync: iso(2 * 60e3) }, children: [] })
  const trees = {
    acc1: [folder('acc1', 'f1-inbox', 'Inbox', 'inbox', 2, 214), folder('acc1', 'f1-drafts', 'Drafts', 'drafts', 0, 1), folder('acc1', 'f1-sent', 'Sent', 'sent', 0, 388), folder('acc1', 'f1-archive', 'Archive', 'archive', 0, 1520), folder('acc1', 'f1-spam', 'Spam', 'spam', 0, 4), folder('acc1', 'f1-trash', 'Trash', 'trash', 0, 12)],
    acc2: [folder('acc2', 'f2-inbox', 'Inbox', 'inbox', 1, 96), folder('acc2', 'f2-sent', 'Sent', 'sent', 0, 140), folder('acc2', 'f2-archive', 'Archive', 'archive', 0, 610), folder('acc2', 'f2-trash', 'Trash', 'trash', 0, 3)],
  }
  const p = (name, email) => ({ name, email })
  const sam = p('Sam Rivera', 'sam.rivera@example.org'), priya = p('Priya Shah', 'priya@example.net'), jamie = p('Jamie Chen', 'jamie.chen@example.com')
  const chat = (id, subject, snippet, people, ago, o = {}) => ({ threadId: id, threadKey: id, subject, snippet, messageCount: o.count ?? 3, unreadCount: o.unread ?? 0, hasAttachments: !!o.att, isStarred: false, latestDate: iso(ago), participants: [...people, me], recipients: [], messageIds: [id + '-m'], isEncrypted: false, accountId: 'acc1', accountName: 'Personal', accountColor: '#88c0d0', folderId: 'f1-inbox', senderEmail: o.sender ?? '', isPinned: !!o.pinned, isLowPriority: !!o.low })
  const pinned = [
    chat('t-trip', 'Weekend trip to the coast', 'Booking the cabin tonight. Sunday dinner is on me.', [priya, sam], 25 * 60e3, { pinned: true, count: 6 }),
    chat('t-mum', 'Sunday lunch', 'Lovely, see you at one. Bring the lemon tart!', [p('Mum', 'mum@example.com')], 3 * H, { pinned: true }),
  ]
  const regular = [
    chat('t-jamie', 'Book club: October pick', 'Has anyone actually finished it yet? I am on chapter nine.', [jamie, p('Lena Fischer', 'lena@example.de')], 40 * 60e3, { unread: 2, count: 6 }),
    chat('t-omar', 'Bike for sale', 'Still available! Happy to meet Saturday morning if that works.', [p('Omar Haddad', 'omar.h@example.org')], 2 * H, { count: 2 }),
    chat('t-dentist', 'Appointment reminder', 'Your check-up is on Tuesday 14 October at 9:30 AM.', [p('Harbour Dental', 'reception@harbourdental.example')], 5 * H, { count: 1 }),
    chat('t-kim', 'Photos from the wedding', 'Uploaded the whole album, there are some great ones of you two.', [p('Kim Park', 'kim.park@example.com')], 26 * H, { att: true, count: 4 }),
    chat('t-noah', 'Running club Thursday', 'Same route as last week? I can bring the new route map.', [p('Noah Williams', 'noah.w@example.net'), p('Ines Duarte', 'ines@example.pt')], 2 * D, { count: 8 }),
    chat('t-library', 'Your reservation is ready', 'The book you reserved is ready for collection until Friday.', [p('City Library', 'noreply@library.example')], 3 * D, { count: 1 }),
    chat('t-grace', 'Recipe you asked for', 'Here it is. The trick is to toast the spices first.', [p('Grace Okafor', 'grace.okafor@example.com')], 4 * D, { count: 3 }),
  ]
  const low = [
    chat('sender:news@trailmag.example', 'Ten autumn hikes worth the drive', 'This week: coastal paths, quiet ridges and where to find good coffee.', [p('Trail Magazine', 'news@trailmag.example')], 6 * H, { low: true, sender: 'news@trailmag.example', count: 12 }),
    chat('sender:offers@grocer.example', 'Your weekly offers', 'Fresh deals on seasonal fruit and vegetables.', [p('Corner Grocer', 'offers@grocer.example')], 30 * H, { low: true, sender: 'offers@grocer.example', count: 20 }),
  ]
  // Like the backend, the all section includes Low priority chats; the list groups them by isLowPriority.
  const lists = { all: [...pinned, ...regular, ...low], unread: regular.filter((c) => c.unreadCount), low, snoozed: [] }
  const msg = (i, from, to, cc, ago, body, mine) => ({ id: 'trip-' + i, accountId: 'acc1', folderId: mine ? 'f1-sent' : 'f1-inbox', uid: i, messageId: `<trip-${i}@example>`, threadId: 't-trip', subject: i ? 'Re: Weekend trip to the coast' : 'Weekend trip to the coast', fromName: from.name, fromEmail: from.email, toList: JSON.stringify(to), ccList: JSON.stringify(cc), date: iso(ago), snippet: body.slice(0, 80), isRead: true, isStarred: false, isAnswered: false, isForwarded: false, isDraft: false, isDeleted: false, size: 2000, hasAttachments: false, bodyText: body, bodyFetched: true, chat: { text: body, hasQuoted: false, isRich: false }, readReceiptHandled: true, mine })
  const trip = [
    msg(0, priya, [me, sam], [], 26 * H, 'Still on for the coast the weekend after next? Found a cabin by the lighthouse 🌊'),
    msg(1, me, [priya, sam], [], 25 * H, 'Yes please! I can drive. Friday night or Saturday morning?', true),
    msg(2, sam, [me, priya], [], 22 * H, 'Saturday works for me. I’ll bring board games and the good coffee ☕'),
    msg(3, me, [priya, sam], [], 60 * 60e3, 'Saturday at 8 then. Priya, want a lift?', true),
    msg(4, priya, [me, sam], [], 30 * 60e3, 'Yes please!'),
    msg(5, priya, [me, sam], [], 25 * 60e3, 'Booking the cabin tonight. Sunday dinner is on me.'),
  ]
  const convo = (id) => {
    const c = [...lists.all, ...low].find((x) => x.threadId === id) ?? pinned[0]
    const messages = id === 't-trip' ? trip : [msg(9, c.participants[0], [me], [], 60 * 60e3, c.snippet)].map((m) => ({ ...m, threadId: id, subject: c.subject }))
    return { ...c, messages, messageCount: messages.length }
  }
  const ui = { selectedAccountId: 'acc1', selectedFolderId: 'f1-inbox', selectedFolderName: 'Inbox', selectedFolderType: 'inbox', selectedThreadId: 't-trip', selectedConversationAccountId: 'acc1', selectedConversationFolderId: 'f1-inbox', sidebarWidth: 184, listWidth: 340, expandedAccounts: { acc1: true, acc2: true }, unifiedInboxExpanded: true, collapsedFolders: {}, activeExtension: 'mail', collapsedSidebars: { calendar: false, contacts: false, mail: true }, chatGroupsExpanded: { low: false }, chatGroupsShowFewer: { chats: false, low: false } }
  Object.assign(A, {
    GetUIState: async () => ui,
    GetAccounts: async () => accounts,
    GetFolderTree: async (id) => trees[id] ?? [],
    GetFolders: async (id) => (trees[id] ?? []).map((t) => t.folder),
    GetIdentities: async (acc) => [{ id: 'id-' + acc, accountId: acc, email: acc === 'acc1' ? me.email : 'alex.morgan@northwind.example', name: me.name, isDefault: true, signatureEnabled: false, signatureForNew: false }],
    GetUnifiedInboxUnreadCount: async () => 3,
    GetChats: async (scope, section, off, lim) => (lists[section] ?? []).slice(off, off + lim),
    GetChatCount: async (scope, section) => (lists[section] ?? []).length,
    SearchChats: async () => [], SearchChatsUnifiedInbox: async () => [],
    GetConversation: async (id) => convo(id),
    GetSenderChat: async (acc, sender) => convo('sender:' + sender),
    GetChatDraft: async () => null,
  })
  return 'ok'
}
