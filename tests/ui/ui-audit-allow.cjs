// Known findings the UI audit tolerates. Each entry names the rule, an element pattern, the configurations it
// applies to and why. Product defects stay listed only while their ISSUES.md entry is open; delete the entry
// when the issue is fixed so the audit guards the fix.
// { rule, element: RegExp over the finding's element descriptor, landmark?: RegExp over a keyboard finding's
//   closest landmark (ui-audit.cjs), when?: (config) => bool, reason }
module.exports = []
