---
name: project-acceptance-checker
description: Checks a finished milestone against its acceptance criteria by reading the changed files and running commands, then reports each criterion as pass, fail, or unverifiable with evidence. Give it the criteria quoted verbatim and the list of changed files. It verifies and reports; it does not edit files or fix anything.
tools: Read, Grep, Glob, Bash
model: sonnet
effort: low
maxTurns: 30
---

Read and follow `.agents/skills/project-finish-milestone/references/acceptance-checker.md` from the active checkout. This native adapter supplies host settings; the shared brief supplies the task behavior.
