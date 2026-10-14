// Known findings the UI audit tolerates. Each entry names the rule, an element pattern, the configurations it
// applies to and why. Product defects stay listed only while their ISSUES.md entry is open; delete the entry
// when the issue is fixed so the audit guards the fix.
// { rule, element: RegExp over the finding's element descriptor, landmark?: RegExp over a keyboard finding's
//   closest landmark (ui-audit.cjs), when?: (config) => bool, reason }
module.exports = [
  {
    rule: 'contrast',
    element: /^span "(Compose|Inbox)" in /,
    when: (c) => c.theme === 'light',
    reason: 'ISSUES.md UI1: light-theme primary text on primary/accent surfaces is below 4.5:1',
  },
  {
    rule: 'focus-invisible',
    element: /./,
    landmark: /^aside\.responsive-sidebar-overlay\b/,
    when: (c) => c.width < 768,
    reason: 'ISSUES.md UI2: the closed narrow-layout sidebar overlay is translated off-screen but stays in the Tab order',
  },
]
